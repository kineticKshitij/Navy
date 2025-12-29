// +build linux

package ipsec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/strongswan/govici/vici"
)

const (
	swanctlConfigPath = "/etc/swanctl/swanctl.conf"
	swanctlConfDir    = "/etc/swanctl"
	viciSocket        = "/var/run/charon.vici"
)

// LinuxManager implements IPsecManager for Linux using strongSwan
type LinuxManager struct {
	session *vici.Session
}

// newLinuxManager creates a new Linux IPsec manager
func newLinuxManager() (IPsecManager, error) {
	// Check if strongSwan is installed
	if _, err := exec.LookPath("swanctl"); err != nil {
		return nil, fmt.Errorf("strongSwan not found: please install strongswan-swanctl package")
	}

	// Create manager instance
	mgr := &LinuxManager{}

	return mgr, nil
}

// Initialize performs platform-specific initialization
func (m *LinuxManager) Initialize(ctx context.Context) error {
	// Ensure swanctl directory exists
	if err := os.MkdirAll(swanctlConfDir, 0755); err != nil {
		return fmt.Errorf("failed to create swanctl directory: %w", err)
	}

	// Try to connect to VICI socket
	session, err := vici.NewSession()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to connect to VICI socket, will retry on demand")
		// Don't fail initialization - strongSwan might not be running yet
		return nil
	}
	m.session = session

	log.Info().Msg("Linux IPsec manager initialized with strongSwan/VICI")
	return nil
}

// ensureSession ensures VICI session is connected
func (m *LinuxManager) ensureSession() error {
	if m.session != nil {
		return nil
	}

	session, err := vici.NewSession()
	if err != nil {
		return fmt.Errorf("failed to connect to VICI socket: %w (is strongSwan running?)", err)
	}
	m.session = session
	return nil
}

// CreateTunnel creates a new IPsec tunnel
func (m *LinuxManager) CreateTunnel(ctx context.Context, config TunnelConfig) error {
	if err := m.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Generate swanctl configuration
	if err := m.generateSwanctlConfig(config); err != nil {
		return fmt.Errorf("failed to generate configuration: %w", err)
	}

	// Load configuration using swanctl
	if err := m.loadSwanctlConfig(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Optionally start tunnel immediately
	if config.AutoStart {
		if err := m.StartTunnel(ctx, config.Name); err != nil {
			log.Warn().Err(err).Str("tunnel", config.Name).Msg("Failed to auto-start tunnel")
		}
	}

	log.Info().Str("tunnel", config.Name).Msg("Tunnel created successfully")
	return nil
}

// generateSwanctlConfig generates swanctl.conf for the tunnel
func (m *LinuxManager) generateSwanctlConfig(config TunnelConfig) error {
	// Template for swanctl.conf
	const swanctlTemplate = `
connections {
    {{.Name}} {
        version = {{.IKEVersion}}
        local_addrs = {{.LocalAddress}}
        remote_addrs = {{.RemoteAddress}}
        {{if .LocalID}}local {
            id = {{.LocalID}}
        }{{end}}
        {{if .RemoteID}}remote {
            id = {{.RemoteID}}
        }{{end}}
        
        local {
            {{if eq .AuthType "psk"}}auth = psk{{else}}auth = pubkey
            certs = {{.CertPath}}{{end}}
        }
        
        remote {
            {{if eq .AuthType "psk"}}auth = psk{{else}}auth = pubkey{{end}}
        }
        
        children {
            {{.Name}}-child {
                {{if or (eq .Mode "esp-tunnel") (eq .Mode "esp-ah-tunnel")}}mode = tunnel{{else}}mode = transport{{end}}
                {{range .TrafficSelectors}}local_ts = {{.LocalSubnet}}
                remote_ts = {{.RemoteSubnet}}
                {{end}}
                esp_proposals = {{.ESPProposal}}
                {{if .UseAH}}ah_proposals = {{.AHProposal}}{{end}}
                dpd_action = {{.DPDAction}}
                life_time = {{.Lifetime}}s
                rekey_time = {{.RekeyTime}}s
                {{if .AutoStart}}start_action = start{{else}}start_action = trap{{end}}
            }
        }
        
        dpd_delay = {{.DPDDelay}}s
    }
}

{{if eq .AuthType "psk"}}
secrets {
    ike-{{.Name}} {
        {{if .LocalID}}id-local = {{.LocalID}}{{end}}
        {{if .RemoteID}}id-remote = {{.RemoteID}}{{end}}
        secret = "{{.Secret}}"
    }
}
{{end}}
`

	tmpl, err := template.New("swanctl").Parse(swanctlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template data
	data := map[string]interface{}{
		"Name":          config.Name,
		"IKEVersion":    convertIKEVersion(config.Crypto.IKEVersion),
		"LocalAddress":  config.LocalAddress,
		"RemoteAddress": config.RemoteAddress,
		"LocalID":       config.LocalID,
		"RemoteID":      config.RemoteID,
		"Mode":          config.Mode,
		"AuthType":      config.Auth.Type,
		"Secret":        config.Auth.Secret,
		"CertPath":      config.Auth.CertPath,
		"ESPProposal":   buildESPProposal(config.Crypto),
		"AHProposal":    buildAHProposal(config.Crypto),
		"UseAH":         config.Mode == ModeAHTunnel || config.Mode == ModeAHTransport || config.Mode == ModeESPAHTunnel,
		"DPDDelay":      int(config.DPD.Delay.Seconds()),
		"DPDAction":     config.DPD.Action,
		"Lifetime":      int(config.Crypto.Lifetime.Seconds()),
		"RekeyTime":     int(config.Crypto.Lifetime.Seconds() * 9 / 10), // Rekey at 90% of lifetime
		"AutoStart":     config.AutoStart,
		"TrafficSelectors": config.TrafficSelectors,
	}

	// Write configuration file
	configPath := filepath.Join(swanctlConfDir, fmt.Sprintf("conf.d/%s.conf", config.Name))
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// loadSwanctlConfig loads configuration using swanctl
func (m *LinuxManager) loadSwanctlConfig() error {
	cmd := exec.Command("swanctl", "--load-all")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load swanctl config: %w: %s", err, output)
	}
	return nil
}

// StartTunnel initiates the IPsec tunnel
func (m *LinuxManager) StartTunnel(ctx context.Context, name string) error {
	if err := m.ensureSession(); err != nil {
		// Fallback to command line
		return m.startTunnelCLI(name)
	}

	// Use VICI to initiate connection
	childName := fmt.Sprintf("%s-child", name)
	msg := vici.NewMessage()
	if err := msg.Set("child", childName); err != nil {
		return fmt.Errorf("failed to set child name: %w", err)
	}

	_, err := m.session.StreamedCommandRequest("initiate", "initiate-event", msg)
	if err != nil {
		return fmt.Errorf("failed to initiate tunnel: %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel initiated")
	return nil
}

// startTunnelCLI starts tunnel using CLI as fallback
func (m *LinuxManager) startTunnelCLI(name string) error {
	childName := fmt.Sprintf("%s-child", name)
	cmd := exec.Command("swanctl", "--initiate", "--child", childName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start tunnel: %w: %s", err, output)
	}
	return nil
}

// StopTunnel terminates the IPsec tunnel
func (m *LinuxManager) StopTunnel(ctx context.Context, name string) error {
	if err := m.ensureSession(); err != nil {
		return m.stopTunnelCLI(name)
	}

	childName := fmt.Sprintf("%s-child", name)
	msg := vici.NewMessage()
	if err := msg.Set("child", childName); err != nil {
		return fmt.Errorf("failed to set child name: %w", err)
	}

	if _, err := m.session.CommandRequest("terminate", msg); err != nil {
		return fmt.Errorf("failed to terminate tunnel: %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel terminated")
	return nil
}

// stopTunnelCLI stops tunnel using CLI as fallback
func (m *LinuxManager) stopTunnelCLI(name string) error {
	childName := fmt.Sprintf("%s-child", name)
	cmd := exec.Command("swanctl", "--terminate", "--child", childName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w: %s", err, output)
	}
	return nil
}

// DeleteTunnel removes an existing IPsec tunnel
func (m *LinuxManager) DeleteTunnel(ctx context.Context, name string) error {
	// Stop tunnel first if running
	_ = m.StopTunnel(ctx, name)

	// Remove configuration file
	configPath := filepath.Join(swanctlConfDir, fmt.Sprintf("conf.d/%s.conf", name))
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	// Reload configuration
	if err := m.loadSwanctlConfig(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload config after deletion")
	}

	log.Info().Str("tunnel", name).Msg("Tunnel deleted")
	return nil
}

// UpdateTunnel updates an existing tunnel configuration
func (m *LinuxManager) UpdateTunnel(ctx context.Context, config TunnelConfig) error {
	// For strongSwan, update is essentially delete + create
	if err := m.DeleteTunnel(ctx, config.Name); err != nil {
		log.Warn().Err(err).Msg("Failed to delete existing tunnel during update")
	}
	return m.CreateTunnel(ctx, config)
}

// GetTunnelStatus retrieves current status of a tunnel
func (m *LinuxManager) GetTunnelStatus(ctx context.Context, name string) (*TunnelStatus, error) {
	if err := m.ensureSession(); err != nil {
		return m.getTunnelStatusCLI(name)
	}

	// Use VICI to list SAs
	msg := vici.NewMessage()
	msgs, err := m.session.StreamedCommandRequest("list-sas", "list-sa", msg)
	if err != nil {
		return nil, fmt.Errorf("failed to list SAs: %w", err)
	}

	// Parse SA information
	status := &TunnelStatus{
		Name:  name,
		State: StateDown,
	}

	for _, saMsg := range msgs {
		// Parse SA message - check if connection exists
		if val := saMsg.Get("name"); val != nil {
			if connName, ok := val.(string); ok && connName == name {
				status.State = StateEstablished
				if bytesIn, ok := saMsg.Get("bytes-in").(int); ok {
					status.BytesIn = uint64(bytesIn)
				}
				if bytesOut, ok := saMsg.Get("bytes-out").(int); ok {
					status.BytesOut = uint64(bytesOut)
				}
				break
			}
		}
	}

	return status, nil
}

// getTunnelStatusCLI gets status using CLI as fallback
func (m *LinuxManager) getTunnelStatusCLI(name string) (*TunnelStatus, error) {
	cmd := exec.Command("swanctl", "--list-sas", "--ike", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &TunnelStatus{Name: name, State: StateDown}, nil
	}

	// Basic parsing - in production, use proper parsing
	status := &TunnelStatus{
		Name:  name,
		State: StateDown,
	}

	if len(output) > 0 && string(output) != "" {
		status.State = StateEstablished
	}

	return status, nil
}

// ListTunnels returns all configured tunnels
func (m *LinuxManager) ListTunnels(ctx context.Context) ([]TunnelStatus, error) {
	// List all .conf files in swanctl conf.d directory
	confDir := filepath.Join(swanctlConfDir, "conf.d")
	entries, err := os.ReadDir(confDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []TunnelStatus{}, nil
		}
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var tunnels []TunnelStatus
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".conf" {
			name := entry.Name()[:len(entry.Name())-5] // Remove .conf extension
			status, err := m.GetTunnelStatus(ctx, name)
			if err != nil {
				log.Warn().Err(err).Str("tunnel", name).Msg("Failed to get tunnel status")
				continue
			}
			tunnels = append(tunnels, *status)
		}
	}

	return tunnels, nil
}

// GetStatistics retrieves traffic statistics
func (m *LinuxManager) GetStatistics(ctx context.Context, name string) (*TrafficStats, error) {
	status, err := m.GetTunnelStatus(ctx, name)
	if err != nil {
		return nil, err
	}

	return &TrafficStats{
		BytesIn:    status.BytesIn,
		BytesOut:   status.BytesOut,
		PacketsIn:  status.PacketsIn,
		PacketsOut: status.PacketsOut,
		Timestamp:  time.Now(),
	}, nil
}

// GetSAInfo retrieves Security Association information
func (m *LinuxManager) GetSAInfo(ctx context.Context, name string) ([]SAInfo, error) {
	// Placeholder - full implementation would parse VICI SA details
	return []SAInfo{}, nil
}

// ValidateConfig validates tunnel configuration
func (m *LinuxManager) ValidateConfig(config TunnelConfig) error {
	if config.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if config.LocalAddress == "" {
		return fmt.Errorf("local address is required")
	}
	if config.RemoteAddress == "" {
		return fmt.Errorf("remote address is required")
	}
	if len(config.TrafficSelectors) == 0 {
		return fmt.Errorf("at least one traffic selector is required")
	}
	if config.Auth.Type == AuthPSK && config.Auth.Secret == "" {
		return fmt.Errorf("PSK secret is required for PSK authentication")
	}
	return nil
}

// Cleanup performs platform-specific cleanup
func (m *LinuxManager) Cleanup(ctx context.Context) error {
	if m.session != nil {
		m.session.Close()
	}
	return nil
}

// Helper functions

func convertIKEVersion(version IKEVersion) int {
	if version == IKEv1 {
		return 1
	}
	return 2
}

func buildESPProposal(crypto CryptoConfig) string {
	return fmt.Sprintf("%s-%s-%s", crypto.Encryption, crypto.Integrity, crypto.DHGroup)
}

func buildAHProposal(crypto CryptoConfig) string {
	return fmt.Sprintf("%s-%s", crypto.Integrity, crypto.DHGroup)
}
