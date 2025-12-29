package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/kardianos/service"
	"github.com/spf13/viper"
	"github.com/swavlamban/ipsec-manager/internal/ipsec"
	"github.com/swavlamban/ipsec-manager/internal/policy"
)

// Agent represents the IPsec management agent
type Agent struct {
	id            string
	manager       ipsec.IPsecManager
	serverURL     string
	syncInterval  time.Duration
	healthInterval time.Duration
	httpClient    *http.Client
	
	currentPolicies []policy.Policy
	currentTunnels  map[string]ipsec.TunnelConfig
	mu              sync.RWMutex
	
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New creates a new agent instance
func New(manager ipsec.IPsecManager) (*Agent, error) {
	serverURL := viper.GetString("server.url")
	if serverURL == "" {
		return nil, fmt.Errorf("server URL not configured")
	}

	syncInterval, err := time.ParseDuration(viper.GetString("agent.sync_interval"))
	if err != nil {
		syncInterval = 60 * time.Second
	}

	healthInterval, err := time.ParseDuration(viper.GetString("agent.health_check_interval"))
	if err != nil {
		healthInterval = 10 * time.Second
	}

	timeout, err := time.ParseDuration(viper.GetString("server.timeout"))
	if err != nil {
		timeout = 30 * time.Second
	}

	// Get or generate peer ID
	peerID := viper.GetString("peer.id")
	if peerID == "" {
		peerID = uuid.New().String()
		log.Info().Str("peer_id", peerID).Msg("Generated new peer ID")
	}

	return &Agent{
		id:              peerID,
		manager:         manager,
		serverURL:       serverURL,
		syncInterval:    syncInterval,
		healthInterval:  healthInterval,
		currentTunnels:  make(map[string]ipsec.TunnelConfig),
		stopCh:          make(chan struct{}),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	log.Info().
		Str("peer_id", a.id).
		Str("server", a.serverURL).
		Dur("sync_interval", a.syncInterval).
		Msg("Starting agent")

	// Register with server
	if err := a.register(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to register with server (will retry)")
	}

	// Initial policy sync
	if err := a.syncPolicies(ctx); err != nil {
		log.Warn().Err(err).Msg("Initial policy sync failed (will retry)")
	}

	// Start background goroutines
	a.wg.Add(3)
	go a.policySyncLoop(ctx)
	go a.healthCheckLoop(ctx)
	go a.watchdogLoop(ctx)

	return nil
}

// Stop stops the agent
func (a *Agent) Stop(ctx context.Context) error {
	log.Info().Msg("Stopping agent")
	close(a.stopCh)
	
	// Wait for goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Agent stopped cleanly")
	case <-time.After(10 * time.Second):
		log.Warn().Msg("Agent shutdown timed out")
	}

	return nil
}

// register registers the agent with the server
func (a *Agent) register(ctx context.Context) error {
	hostname, _ := os.Hostname()
	
	peerInfo := policy.PeerInfo{
		ID:           a.id,
		Hostname:     hostname,
		Platform:     runtime.GOOS,
		IPAddress:    a.getLocalIP(),
		Version:      "0.1.0", // TODO: Get from build info
		RegisteredAt: time.Now(),
		LastSeenAt:   time.Now(),
		Status:       policy.PeerStatusOnline,
		Tags:         viper.GetStringSlice("peer.tags"),
		Metadata:     map[string]string{
			"arch": runtime.GOARCH,
		},
	}

	jsonData, err := json.Marshal(peerInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal peer info: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.serverURL+"/api/peers/register", 
		bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s: %s", resp.Status, body)
	}

	log.Info().Str("peer_id", a.id).Msg("Registered with server")
	return nil
}

// syncPolicies fetches and applies policies from the server
func (a *Agent) syncPolicies(ctx context.Context) error {
	log.Debug().Msg("Syncing policies")

	req, err := http.NewRequestWithContext(ctx, "GET", 
		fmt.Sprintf("%s/api/policies?peer_id=%s", a.serverURL, a.id), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch policies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch policies: %s", resp.Status)
	}

	var policies []policy.Policy
	if err := json.NewDecoder(resp.Body).Decode(&policies); err != nil {
		return fmt.Errorf("failed to decode policies: %w", err)
	}

	log.Info().Int("count", len(policies)).Msg("Fetched policies")

	// Apply policies
	if err := a.applyPolicies(ctx, policies); err != nil {
		return fmt.Errorf("failed to apply policies: %w", err)
	}

	a.mu.Lock()
	a.currentPolicies = policies
	a.mu.Unlock()

	return nil
}

// applyPolicies applies the fetched policies
func (a *Agent) applyPolicies(ctx context.Context, policies []policy.Policy) error {
	// Extract all tunnel configurations
	var desiredTunnels = make(map[string]ipsec.TunnelConfig)
	
	for _, pol := range policies {
		if !pol.Enabled {
			continue
		}
		for _, tunnel := range pol.Tunnels {
			desiredTunnels[tunnel.Name] = tunnel
		}
	}

	// Get current tunnels
	currentTunnels, err := a.manager.ListTunnels(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list current tunnels")
	}

	currentNames := make(map[string]bool)
	for _, t := range currentTunnels {
		currentNames[t.Name] = true
	}

	// Create or update tunnels
	for name, tunnel := range desiredTunnels {
		if currentNames[name] {
			// Update existing
			if err := a.manager.UpdateTunnel(ctx, tunnel); err != nil {
				log.Error().Err(err).Str("tunnel", name).Msg("Failed to update tunnel")
				continue
			}
			log.Info().Str("tunnel", name).Msg("Updated tunnel")
		} else {
			// Create new
			if err := a.manager.CreateTunnel(ctx, tunnel); err != nil {
				log.Error().Err(err).Str("tunnel", name).Msg("Failed to create tunnel")
				continue
			}
			log.Info().Str("tunnel", name).Msg("Created tunnel")
		}
	}

	// Delete removed tunnels
	for name := range currentNames {
		if _, exists := desiredTunnels[name]; !exists {
			if err := a.manager.DeleteTunnel(ctx, name); err != nil {
				log.Error().Err(err).Str("tunnel", name).Msg("Failed to delete tunnel")
				continue
			}
			log.Info().Str("tunnel", name).Msg("Deleted tunnel")
		}
	}

	a.mu.Lock()
	a.currentTunnels = desiredTunnels
	a.mu.Unlock()

	return nil
}

// policySyncLoop periodically syncs policies
func (a *Agent) policySyncLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.syncPolicies(ctx); err != nil {
				log.Error().Err(err).Msg("Policy sync failed")
			}
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// healthCheckLoop periodically checks tunnel health
func (a *Agent) healthCheckLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.checkHealth(ctx)
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkHealth checks the health of all tunnels
func (a *Agent) checkHealth(ctx context.Context) {
	a.mu.RLock()
	tunnels := make(map[string]ipsec.TunnelConfig)
	for k, v := range a.currentTunnels {
		tunnels[k] = v
	}
	a.mu.RUnlock()

	for name := range tunnels {
		status, err := a.manager.GetTunnelStatus(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("tunnel", name).Msg("Failed to get tunnel status")
			continue
		}

		if status.State == ipsec.StateError {
			log.Error().Str("tunnel", name).Str("error", status.ErrorMessage).Msg("Tunnel in error state")
		}
	}
}

// watchdogLoop monitors and restarts failed tunnels
func (a *Agent) watchdogLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.watchdogCheck(ctx)
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// watchdogCheck checks and restarts failed tunnels
func (a *Agent) watchdogCheck(ctx context.Context) {
	a.mu.RLock()
	tunnels := make(map[string]ipsec.TunnelConfig)
	for k, v := range a.currentTunnels {
		tunnels[k] = v
	}
	a.mu.RUnlock()

	for name, config := range tunnels {
		if !config.AutoStart {
			continue
		}

		status, err := a.manager.GetTunnelStatus(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("tunnel", name).Msg("Failed to get tunnel status")
			continue
		}

		if status.State == ipsec.StateDown || status.State == ipsec.StateError {
			log.Warn().Str("tunnel", name).Msg("Tunnel down, attempting restart")
			if err := a.manager.StartTunnel(ctx, name); err != nil {
				log.Error().Err(err).Str("tunnel", name).Msg("Failed to restart tunnel")
			} else {
				log.Info().Str("tunnel", name).Msg("Tunnel restarted successfully")
			}
		}
	}
}

// getLocalIP attempts to get the local IP address
func (a *Agent) getLocalIP() string {
	// Simplified implementation - in production, use proper network detection
	return "127.0.0.1"
}

// Service wraps the agent as a system service
type Service struct {
	agent  *Agent
	logger service.Logger
}

// NewService creates a new service wrapper
func NewService() (service.Service, error) {
	svcConfig := &service.Config{
		Name:        "ipsec-agent",
		DisplayName: "IPsec Management Agent",
		Description: "Cross-platform IPsec tunnel management agent for SWAVLAMBAN",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// program implements the service.Interface
type program struct{}

func (p *program) Start(s service.Service) error {
	log.Info().Msg("Service starting")
	go p.run()
	return nil
}

func (p *program) run() {
	// Implementation would start the agent here
	log.Info().Msg("Service running")
}

func (p *program) Stop(s service.Service) error {
	log.Info().Msg("Service stopping")
	return nil
}
