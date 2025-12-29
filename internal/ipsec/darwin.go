// +build darwin

package ipsec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// DarwinManager implements IPsecManager for macOS
type DarwinManager struct {
	configDir string
}

// newDarwinManager creates a new macOS IPsec manager
func newDarwinManager() (IPsecManager, error) {
	// Check if scutil is available
	if _, err := exec.LookPath("scutil"); err != nil {
		return nil, fmt.Errorf("scutil not found: %w", err)
	}

	configDir := "/etc/ipsec"
	return &DarwinManager{
		configDir: configDir,
	}, nil
}

// Initialize performs platform-specific initialization
func (m *DarwinManager) Initialize(ctx context.Context) error {
	// Create config directory
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	log.Info().Msg("macOS IPsec manager initialized")
	return nil
}

// CreateTunnel creates a new IPsec tunnel
func (m *DarwinManager) CreateTunnel(ctx context.Context, config TunnelConfig) error {
	if err := m.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create VPN configuration using networksetup
	// Note: macOS VPN configuration is more complex and may require
	// VPN client applications or configuration profiles for full IPsec support

	// For basic IPsec, we create a racoon configuration
	if err := m.createRacoonConfig(config); err != nil {
		return fmt.Errorf("failed to create configuration: %w", err)
	}

	log.Info().Str("tunnel", config.Name).Msg("Tunnel created successfully")
	return nil
}

// createRacoonConfig creates racoon configuration files
func (m *DarwinManager) createRacoonConfig(config TunnelConfig) error {
	// Note: Modern macOS has deprecated racoon in favor of IKEv2
	// This is a simplified implementation
	// Production implementation should use VPN configuration profiles

	configPath := filepath.Join(m.configDir, fmt.Sprintf("%s.conf", config.Name))
	
	configContent := fmt.Sprintf(`# IPsec tunnel configuration: %s
remote %s {
	exchange_mode main;
	doi ipsec_doi;
	situation identity_only;
	
	%s
	
	proposal {
		encryption_algorithm %s;
		hash_algorithm %s;
		authentication_method %s;
		dh_group %s;
	}
}

sainfo address %s any address %s any {
	pfs_group %s;
	encryption_algorithm %s;
	authentication_algorithm %s;
	compression_algorithm deflate;
}
`,
		config.Name,
		config.RemoteAddress,
		m.buildAuthConfig(config.Auth),
		m.convertEncryption(config.Crypto.Encryption),
		m.convertIntegrity(config.Crypto.Integrity),
		m.convertAuthMethod(config.Auth.Type),
		m.convertDHGroup(config.Crypto.DHGroup),
		config.LocalAddress,
		config.RemoteAddress,
		m.convertDHGroup(config.Crypto.DHGroup),
		m.convertEncryption(config.Crypto.Encryption),
		m.convertIntegrity(config.Crypto.Integrity),
	)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	return nil
}

// buildAuthConfig builds authentication configuration
func (m *DarwinManager) buildAuthConfig(auth AuthConfig) string {
	if auth.Type == AuthPSK {
		return fmt.Sprintf("my_identifier address;\n\tpeers_identifier address;\n\tpre_shared_key \"%s\";", auth.Secret)
	}
	return fmt.Sprintf("certificate_type x509 \"%s\" \"%s\";\n\tca_type x509 \"%s\";",
		auth.CertPath, auth.KeyPath, auth.CACertPath)
}

// DeleteTunnel removes an existing IPsec tunnel
func (m *DarwinManager) DeleteTunnel(ctx context.Context, name string) error {
	// Stop tunnel first
	_ = m.StopTunnel(ctx, name)

	// Remove configuration file
	configPath := filepath.Join(m.configDir, fmt.Sprintf("%s.conf", name))
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config: %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel deleted")
	return nil
}

// UpdateTunnel updates an existing tunnel configuration
func (m *DarwinManager) UpdateTunnel(ctx context.Context, config TunnelConfig) error {
	if err := m.DeleteTunnel(ctx, config.Name); err != nil {
		log.Warn().Err(err).Msg("Failed to delete existing tunnel during update")
	}
	return m.CreateTunnel(ctx, config)
}

// StartTunnel initiates the IPsec tunnel
func (m *DarwinManager) StartTunnel(ctx context.Context, name string) error {
	// Use scutil to start VPN connection
	// Note: This requires the connection to be configured in System Preferences
	cmd := exec.Command("scutil", "--nc", "start", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Connection might not exist in System Preferences
		log.Warn().Err(err).Str("output", string(output)).Msg("Failed to start via scutil")
		return fmt.Errorf("failed to start tunnel (may need manual configuration): %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel started")
	return nil
}

// StopTunnel terminates the IPsec tunnel
func (m *DarwinManager) StopTunnel(ctx context.Context, name string) error {
	cmd := exec.Command("scutil", "--nc", "stop", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Warn().Err(err).Str("output", string(output)).Msg("Failed to stop via scutil")
	}

	log.Info().Str("tunnel", name).Msg("Tunnel stopped")
	return nil
}

// GetTunnelStatus retrieves current status of a tunnel
func (m *DarwinManager) GetTunnelStatus(ctx context.Context, name string) (*TunnelStatus, error) {
	cmd := exec.Command("scutil", "--nc", "status", name)
	output, err := cmd.CombinedOutput()

	status := &TunnelStatus{
		Name:  name,
		State: StateDown,
	}

	if err != nil {
		return status, nil
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Connected") {
		status.State = StateEstablished
		status.EstablishedAt = time.Now() // Approximate
	} else if strings.Contains(outputStr, "Connecting") {
		status.State = StateConnecting
	}

	return status, nil
}

// ListTunnels returns all configured tunnels
func (m *DarwinManager) ListTunnels(ctx context.Context) ([]TunnelStatus, error) {
	// List all .conf files
	entries, err := os.ReadDir(m.configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []TunnelStatus{}, nil
		}
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var tunnels []TunnelStatus
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".conf" {
			name := entry.Name()[:len(entry.Name())-5]
			status, err := m.GetTunnelStatus(ctx, name)
			if err != nil {
				log.Warn().Err(err).Str("tunnel", name).Msg("Failed to get tunnel status")
				continue
			}
			tunnels = append(tunnels, *status)
		}
	}

	// Also check scutil for VPN services
	cmd := exec.Command("scutil", "--nc", "list")
	if output, err := cmd.CombinedOutput(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "IPSec") {
				// Parse service name from output
				// Format is typically: * (Connected) <serviceName>
				parts := strings.Split(line, "\"")
				if len(parts) >= 2 {
					serviceName := parts[1]
					// Check if we already have this tunnel
					found := false
					for i, t := range tunnels {
						if t.Name == serviceName {
							found = true
							if strings.Contains(line, "Connected") {
								tunnels[i].State = StateEstablished
							}
							break
						}
					}
					if !found {
						state := StateDown
						if strings.Contains(line, "Connected") {
							state = StateEstablished
						} else if strings.Contains(line, "Connecting") {
							state = StateConnecting
						}
						tunnels = append(tunnels, TunnelStatus{
							Name:  serviceName,
							State: state,
						})
					}
				}
			}
		}
	}

	return tunnels, nil
}

// GetStatistics retrieves traffic statistics
func (m *DarwinManager) GetStatistics(ctx context.Context, name string) (*TrafficStats, error) {
	// macOS doesn't provide easy access to IPsec statistics via command line
	// Would need to use system APIs or netstat parsing
	return &TrafficStats{
		Timestamp: time.Now(),
	}, nil
}

// GetSAInfo retrieves Security Association information
func (m *DarwinManager) GetSAInfo(ctx context.Context, name string) ([]SAInfo, error) {
	// Would require parsing setkey output or using system APIs
	return []SAInfo{}, nil
}

// ValidateConfig validates tunnel configuration
func (m *DarwinManager) ValidateConfig(config TunnelConfig) error {
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
	return nil
}

// Cleanup performs platform-specific cleanup
func (m *DarwinManager) Cleanup(ctx context.Context) error {
	return nil
}

// Helper functions

func (m *DarwinManager) convertEncryption(alg EncryptionAlgorithm) string {
	switch alg {
	case EncryptionAES128, EncryptionAES128GCM:
		return "aes 128"
	case EncryptionAES256, EncryptionAES256GCM:
		return "aes 256"
	case Encryption3DES:
		return "3des"
	default:
		return "aes 256"
	}
}

func (m *DarwinManager) convertIntegrity(alg IntegrityAlgorithm) string {
	switch alg {
	case IntegritySHA1:
		return "sha1"
	case IntegritySHA256:
		return "sha256"
	case IntegritySHA384:
		return "sha384"
	case IntegritySHA512:
		return "sha512"
	default:
		return "sha256"
	}
}

func (m *DarwinManager) convertDHGroup(group DHGroup) string {
	switch group {
	case DHGroupModp1024:
		return "2"
	case DHGroupModp1536:
		return "5"
	case DHGroupModp2048:
		return "14"
	case DHGroupModp3072:
		return "15"
	case DHGroupModp4096:
		return "16"
	case DHGroupECP256:
		return "19"
	case DHGroupECP384:
		return "20"
	default:
		return "14"
	}
}

func (m *DarwinManager) convertAuthMethod(authType AuthType) string {
	if authType == AuthPSK {
		return "pre_shared_key"
	}
	return "rsasig"
}
