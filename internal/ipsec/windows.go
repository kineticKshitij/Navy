// +build windows

package ipsec

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// WindowsManager implements IPsecManager for Windows using PowerShell NetIPsec cmdlets
type WindowsManager struct {
	initialized bool
}

// newWindowsManager creates a new Windows IPsec manager
func newWindowsManager() (IPsecManager, error) {
	// Check if PowerShell is available
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		return nil, fmt.Errorf("PowerShell not found: %w", err)
	}

	return &WindowsManager{}, nil
}

// Initialize performs platform-specific initialization
func (m *WindowsManager) Initialize(ctx context.Context) error {
	// Ensure IPsec services are running
	script := `
		$services = @('IKEEXT', 'PolicyAgent')
		foreach ($svc in $services) {
			$service = Get-Service -Name $svc -ErrorAction SilentlyContinue
			if ($service) {
				if ($service.Status -ne 'Running') {
					Start-Service -Name $svc
				}
				Set-Service -Name $svc -StartupType Automatic
			}
		}
		Write-Output 'Services configured'
	`

	if _, err := m.executePowerShell(script); err != nil {
		return fmt.Errorf("failed to initialize IPsec services: %w", err)
	}

	m.initialized = true
	log.Info().Msg("Windows IPsec manager initialized")
	return nil
}

// executePowerShell executes a PowerShell script and returns output
func (m *WindowsManager) executePowerShell(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("PowerShell execution failed: %w: %s", err, output)
	}
	return string(output), nil
}

// CreateTunnel creates a new IPsec tunnel
func (m *WindowsManager) CreateTunnel(ctx context.Context, config TunnelConfig) error {
	if err := m.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create connection security rules for Windows
	script := m.buildCreateTunnelScript(config)
	if _, err := m.executePowerShell(script); err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	log.Info().Str("tunnel", config.Name).Msg("Tunnel created successfully")
	return nil
}

// buildCreateTunnelScript builds PowerShell script for tunnel creation
func (m *WindowsManager) buildCreateTunnelScript(config TunnelConfig) string {
	// Build encryption algorithm
	encAlg := m.convertEncryptionAlgorithm(config.Crypto.Encryption)
	intAlg := m.convertIntegrityAlgorithm(config.Crypto.Integrity)
	dhGroup := m.convertDHGroup(config.Crypto.DHGroup)

	// Determine tunnel mode
	tunnelMode := "Transport"
	useAH := false
	useESP := true

	switch config.Mode {
	case ModeESPTunnel:
		tunnelMode = "Tunnel"
	case ModeAHTunnel:
		tunnelMode = "Tunnel"
		useAH = true
		useESP = false
	case ModeAHTransport:
		useAH = true
		useESP = false
	case ModeESPAHTunnel:
		tunnelMode = "Tunnel"
		useAH = true
	}

	// Build traffic selector strings
	var localSubnets, remoteSubnets []string
	for _, ts := range config.TrafficSelectors {
		localSubnets = append(localSubnets, ts.LocalSubnet)
		remoteSubnets = append(remoteSubnets, ts.RemoteSubnet)
	}

	script := fmt.Sprintf(`
# Remove existing rules with the same name
Remove-NetIPsecRule -Name '%s' -ErrorAction SilentlyContinue
Remove-NetIPsecMainModeRule -Name '%s-MM' -ErrorAction SilentlyContinue

# Create Phase 1 (Main Mode) proposal
$Phase1Proposal = New-NetIPsecMainModeCryptoProposal -Encryption %s -Hash %s -DHGroup %s

# Create Phase 1 Authentication
$Phase1Auth = New-NetIPsecAuthProposal -Machine -Cert -Authority 'CN=Root' -AuthorityType Root
%s

# Create Phase 1 Main Mode Rule
New-NetIPsecMainModeRule -Name '%s-MM' -DisplayName '%s Main Mode' -MainModeCryptoSet $Phase1Proposal -Phase1AuthSet $Phase1Auth -LocalAddress %s -RemoteAddress %s

# Create Phase 2 (Quick Mode) proposal
$Phase2Proposal = New-NetIPsecQuickModeCryptoProposal -Encapsulation %s -Encryption %s -Hash %s -PfsGroup %s

# Create connection security rule
New-NetIPsecRule -Name '%s' -DisplayName '%s' -Mode %s -LocalAddress @(%s) -RemoteAddress @(%s) -QuickModeCryptoSet $Phase2Proposal -InboundSecurity Require -OutboundSecurity Require -Phase2AuthSet Computer

Write-Output 'Tunnel created successfully'
`,
		config.Name, config.Name,
		encAlg, intAlg, dhGroup,
		m.buildAuthScript(config.Auth),
		config.Name, config.Name,
		config.LocalAddress, config.RemoteAddress,
		m.getEncapsulation(useESP, useAH), encAlg, intAlg, dhGroup,
		config.Name, config.Name,
		tunnelMode,
		m.quoteArray(localSubnets), m.quoteArray(remoteSubnets),
	)

	return script
}

// buildAuthScript builds authentication configuration
func (m *WindowsManager) buildAuthScript(auth AuthConfig) string {
	if auth.Type == AuthPSK {
		// For PSK, we need to create a pre-shared key
		return fmt.Sprintf(`
# Create PSK authentication
$Phase1Auth = New-NetIPsecAuthProposal -Machine -PreSharedKey
# Note: Windows requires PSK to be configured via UI or registry for security
`)
	}
	return `# Certificate-based authentication (default)`
}

// DeleteTunnel removes an existing IPsec tunnel
func (m *WindowsManager) DeleteTunnel(ctx context.Context, name string) error {
	script := fmt.Sprintf(`
Remove-NetIPsecRule -Name '%s' -ErrorAction SilentlyContinue
Remove-NetIPsecMainModeRule -Name '%s-MM' -ErrorAction SilentlyContinue
Write-Output 'Tunnel deleted'
`, name, name)

	if _, err := m.executePowerShell(script); err != nil {
		return fmt.Errorf("failed to delete tunnel: %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel deleted")
	return nil
}

// UpdateTunnel updates an existing tunnel configuration
func (m *WindowsManager) UpdateTunnel(ctx context.Context, config TunnelConfig) error {
	// Delete and recreate
	if err := m.DeleteTunnel(ctx, config.Name); err != nil {
		log.Warn().Err(err).Msg("Failed to delete existing tunnel during update")
	}
	return m.CreateTunnel(ctx, config)
}

// StartTunnel initiates the IPsec tunnel
func (m *WindowsManager) StartTunnel(ctx context.Context, name string) error {
	// Windows IPsec is always active once rules are created
	// We can enable the rule if it was disabled
	script := fmt.Sprintf(`
Enable-NetIPsecRule -Name '%s' -ErrorAction SilentlyContinue
Write-Output 'Tunnel started'
`, name)

	if _, err := m.executePowerShell(script); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel started")
	return nil
}

// StopTunnel terminates the IPsec tunnel
func (m *WindowsManager) StopTunnel(ctx context.Context, name string) error {
	script := fmt.Sprintf(`
Disable-NetIPsecRule -Name '%s' -ErrorAction SilentlyContinue
Write-Output 'Tunnel stopped'
`, name)

	if _, err := m.executePowerShell(script); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	log.Info().Str("tunnel", name).Msg("Tunnel stopped")
	return nil
}

// GetTunnelStatus retrieves current status of a tunnel
func (m *WindowsManager) GetTunnelStatus(ctx context.Context, name string) (*TunnelStatus, error) {
	script := fmt.Sprintf(`
$rule = Get-NetIPsecRule -Name '%s' -ErrorAction SilentlyContinue
$sas = Get-NetIPsecQuickModeSA | Where-Object { $_.Name -like '*%s*' }

$status = @{
	Name = '%s'
	State = 'down'
	Enabled = $false
	BytesIn = 0
	BytesOut = 0
}

if ($rule) {
	$status.Enabled = $rule.Enabled
	if ($sas) {
		$status.State = 'established'
		foreach ($sa in $sas) {
			$status.BytesIn += $sa.InboundBytes
			$status.BytesOut += $sa.OutboundBytes
		}
	}
}

$status | ConvertTo-Json -Compress
`, name, name, name)

	output, err := m.executePowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnel status: %w", err)
	}

	// Parse JSON output
	var result struct {
		Name     string `json:"Name"`
		State    string `json:"State"`
		Enabled  bool   `json:"Enabled"`
		BytesIn  uint64 `json:"BytesIn"`
		BytesOut uint64 `json:"BytesOut"`
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		return nil, fmt.Errorf("failed to parse status: %w", err)
	}

	state := StateDown
	if result.State == "established" {
		state = StateEstablished
	} else if result.Enabled {
		state = StateConnecting
	}

	return &TunnelStatus{
		Name:     result.Name,
		State:    state,
		BytesIn:  result.BytesIn,
		BytesOut: result.BytesOut,
	}, nil
}

// ListTunnels returns all configured tunnels
func (m *WindowsManager) ListTunnels(ctx context.Context) ([]TunnelStatus, error) {
	script := `
$ErrorActionPreference = 'SilentlyContinue'
$rules = Get-NetIPsecRule | Where-Object { $_.DisplayName -notlike '*Windows*' }
$results = @()

if ($rules) {
	foreach ($rule in $rules) {
		$sas = Get-NetIPsecQuickModeSA | Where-Object { $_.Name -like ('*' + $rule.Name + '*') }
		
		$status = @{
			Name = $rule.Name
			State = 'down'
			BytesIn = 0
			BytesOut = 0
		}
		
		if ($sas) {
			$status.State = 'established'
			foreach ($sa in $sas) {
				$status.BytesIn += $sa.InboundBytes
				$status.BytesOut += $sa.OutboundBytes
			}
		} elseif ($rule.Enabled) {
			$status.State = 'connecting'
		}
		
		$results += $status
	}
}

if ($results.Count -eq 0) {
	Write-Output '[]'
} else {
	$results | ConvertTo-Json -Compress
}
`

	output, err := m.executePowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to list tunnels: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return []TunnelStatus{}, nil
	}

	var results []struct {
		Name     string `json:"Name"`
		State    string `json:"State"`
		BytesIn  uint64 `json:"BytesIn"`
		BytesOut uint64 `json:"BytesOut"`
	}

	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, fmt.Errorf("failed to parse tunnels: %w (output: %s)", err, output)
	}

	var tunnels []TunnelStatus
	for _, r := range results {
		state := StateDown
		switch r.State {
		case "established":
			state = StateEstablished
		case "connecting":
			state = StateConnecting
		}

		tunnels = append(tunnels, TunnelStatus{
			Name:     r.Name,
			State:    state,
			BytesIn:  r.BytesIn,
			BytesOut: r.BytesOut,
		})
	}

	return tunnels, nil
}

// GetStatistics retrieves traffic statistics
func (m *WindowsManager) GetStatistics(ctx context.Context, name string) (*TrafficStats, error) {
	status, err := m.GetTunnelStatus(ctx, name)
	if err != nil {
		return nil, err
	}

	return &TrafficStats{
		BytesIn:   status.BytesIn,
		BytesOut:  status.BytesOut,
		Timestamp: time.Now(),
	}, nil
}

// GetSAInfo retrieves Security Association information
func (m *WindowsManager) GetSAInfo(ctx context.Context, name string) ([]SAInfo, error) {
	script := fmt.Sprintf(`
$sas = Get-NetIPsecQuickModeSA | Where-Object { $_.Name -like '*%s*' }
$results = @()

foreach ($sa in $sas) {
	$results += @{
		LocalSPI = $sa.LocalSPI
		RemoteSPI = $sa.RemoteSPI
		Crypto = $sa.EncryptionAlgorithm
		Integrity = $sa.HashAlgorithm
	}
}

$results | ConvertTo-Json -Compress
`, name)

	_, err := m.executePowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to get SA info: %w", err)
	}

	// Parse and return SA info (simplified)
	return []SAInfo{}, nil
}

// ValidateConfig validates tunnel configuration
func (m *WindowsManager) ValidateConfig(config TunnelConfig) error {
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
func (m *WindowsManager) Cleanup(ctx context.Context) error {
	return nil
}

// Helper functions

func (m *WindowsManager) convertEncryptionAlgorithm(alg EncryptionAlgorithm) string {
	switch alg {
	case EncryptionAES128, EncryptionAES128GCM:
		return "AES128"
	case EncryptionAES256, EncryptionAES256GCM:
		return "AES256"
	case Encryption3DES:
		return "3DES"
	default:
		return "AES256"
	}
}

func (m *WindowsManager) convertIntegrityAlgorithm(alg IntegrityAlgorithm) string {
	switch alg {
	case IntegritySHA1:
		return "SHA1"
	case IntegritySHA256:
		return "SHA256"
	case IntegritySHA384:
		return "SHA384"
	case IntegritySHA512:
		return "SHA512"
	default:
		return "SHA256"
	}
}

func (m *WindowsManager) convertDHGroup(group DHGroup) string {
	switch group {
	case DHGroupModp1024:
		return "Group2"
	case DHGroupModp1536:
		return "Group5"
	case DHGroupModp2048:
		return "Group14"
	case DHGroupModp3072:
		return "Group15"
	case DHGroupModp4096:
		return "Group16"
	case DHGroupECP256:
		return "ECP256"
	case DHGroupECP384:
		return "ECP384"
	default:
		return "Group14"
	}
}

func (m *WindowsManager) getEncapsulation(useESP, useAH bool) string {
	if useESP && useAH {
		return "ESP+AH"
	} else if useAH {
		return "AH"
	}
	return "ESP"
}

func (m *WindowsManager) quoteArray(items []string) string {
	var quoted []string
	for _, item := range items {
		quoted = append(quoted, fmt.Sprintf("'%s'", item))
	}
	return strings.Join(quoted, ", ")
}
