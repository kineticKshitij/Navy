package policy

import (
	"fmt"
	"time"

	"github.com/swavlamban/ipsec-manager/internal/ipsec"
)

// Policy represents a complete IPsec policy configuration
type Policy struct {
	ID          string                `json:"id" yaml:"id"`
	Name        string                `json:"name" yaml:"name"`
	Description string                `json:"description,omitempty" yaml:"description,omitempty"`
	Version     int                   `json:"version" yaml:"version"`
	CreatedAt   time.Time             `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at" yaml:"updated_at"`
	Enabled     bool                  `json:"enabled" yaml:"enabled"`
	Tunnels     []ipsec.TunnelConfig  `json:"tunnels" yaml:"tunnels"`
	AppliesTo   []string              `json:"applies_to,omitempty" yaml:"applies_to,omitempty"` // Peer IDs or tags
	Priority    int                   `json:"priority" yaml:"priority"` // Higher priority = applied first
}

// PeerInfo represents information about a registered peer/agent
type PeerInfo struct {
	ID           string            `json:"id" yaml:"id"`
	Hostname     string            `json:"hostname" yaml:"hostname"`
	Platform     string            `json:"platform" yaml:"platform"` // linux, windows, darwin
	IPAddress    string            `json:"ip_address" yaml:"ip_address"`
	Version      string            `json:"version" yaml:"version"` // Agent version
	Tags         []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	LastSeenAt   time.Time         `json:"last_seen_at" yaml:"last_seen_at"`
	RegisteredAt time.Time         `json:"registered_at" yaml:"registered_at"`
	Metadata     map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Status       PeerStatus        `json:"status" yaml:"status"`
}

// PeerStatus represents the current status of a peer
type PeerStatus string

const (
	PeerStatusOnline  PeerStatus = "online"
	PeerStatusOffline PeerStatus = "offline"
	PeerStatusError   PeerStatus = "error"
)

// PolicyEngine handles policy validation and application logic
type PolicyEngine struct {
	validators []PolicyValidator
}

// PolicyValidator is an interface for policy validation rules
type PolicyValidator interface {
	Validate(policy *Policy) error
}

// NewPolicyEngine creates a new policy engine with default validators
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		validators: []PolicyValidator{
			&BasicValidator{},
			&SecurityValidator{},
			&PlatformCompatibilityValidator{},
		},
	}
}

// Validate validates a policy using all registered validators
func (e *PolicyEngine) Validate(policy *Policy) error {
	for _, validator := range e.validators {
		if err := validator.Validate(policy); err != nil {
			return fmt.Errorf("policy validation failed: %w", err)
		}
	}
	return nil
}

// FilterPoliciesForPeer returns policies that apply to a specific peer
func (e *PolicyEngine) FilterPoliciesForPeer(policies []Policy, peer *PeerInfo) []Policy {
	var applicable []Policy
	
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		
		// If no specific peers/tags specified, policy applies to all
		if len(policy.AppliesTo) == 0 {
			applicable = append(applicable, policy)
			continue
		}
		
		// Check if peer ID or any tag matches
		for _, target := range policy.AppliesTo {
			if target == peer.ID || target == "*" {
				applicable = append(applicable, policy)
				break
			}
			
			// Check tags
			for _, tag := range peer.Tags {
				if target == tag {
					applicable = append(applicable, policy)
					break
				}
			}
		}
	}
	
	return applicable
}

// MergeTunnels merges tunnel configurations from multiple policies
// Higher priority policies override lower priority ones
func (e *PolicyEngine) MergeTunnels(policies []Policy) []ipsec.TunnelConfig {
	tunnelMap := make(map[string]ipsec.TunnelConfig)
	
	// Sort by priority (should be done by caller, but we'll handle it)
	// For now, iterate in order assuming they're sorted
	
	for _, policy := range policies {
		for _, tunnel := range policy.Tunnels {
			// If tunnel with same name exists, override if higher priority
			tunnelMap[tunnel.Name] = tunnel
		}
	}
	
	// Convert map to slice
	var tunnels []ipsec.TunnelConfig
	for _, tunnel := range tunnelMap {
		tunnels = append(tunnels, tunnel)
	}
	
	return tunnels
}

// BasicValidator validates basic policy structure
type BasicValidator struct{}

func (v *BasicValidator) Validate(policy *Policy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	
	if len(policy.Tunnels) == 0 {
		return fmt.Errorf("policy must contain at least one tunnel configuration")
	}
	
	// Validate each tunnel
	for i, tunnel := range policy.Tunnels {
		if err := v.validateTunnel(tunnel); err != nil {
			return fmt.Errorf("tunnel %d (%s): %w", i, tunnel.Name, err)
		}
	}
	
	return nil
}

func (v *BasicValidator) validateTunnel(tunnel ipsec.TunnelConfig) error {
	if tunnel.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	
	if tunnel.LocalAddress == "" {
		return fmt.Errorf("local address is required")
	}
	
	if tunnel.RemoteAddress == "" {
		return fmt.Errorf("remote address is required")
	}
	
	if len(tunnel.TrafficSelectors) == 0 {
		return fmt.Errorf("at least one traffic selector is required")
	}
	
	// Validate traffic selectors
	for i, ts := range tunnel.TrafficSelectors {
		if ts.LocalSubnet == "" {
			return fmt.Errorf("traffic selector %d: local subnet is required", i)
		}
		if ts.RemoteSubnet == "" {
			return fmt.Errorf("traffic selector %d: remote subnet is required", i)
		}
	}
	
	return nil
}

// SecurityValidator validates security-related configurations
type SecurityValidator struct{}

func (v *SecurityValidator) Validate(policy *Policy) error {
	for i, tunnel := range policy.Tunnels {
		// Validate authentication
		if tunnel.Auth.Type == ipsec.AuthPSK {
			if tunnel.Auth.Secret == "" {
				return fmt.Errorf("tunnel %d (%s): PSK secret is required", i, tunnel.Name)
			}
			if len(tunnel.Auth.Secret) < 8 {
				return fmt.Errorf("tunnel %d (%s): PSK secret must be at least 8 characters", i, tunnel.Name)
			}
		} else if tunnel.Auth.Type == ipsec.AuthCertificate {
			if tunnel.Auth.CertPath == "" {
				return fmt.Errorf("tunnel %d (%s): certificate path is required", i, tunnel.Name)
			}
			if tunnel.Auth.KeyPath == "" {
				return fmt.Errorf("tunnel %d (%s): private key path is required", i, tunnel.Name)
			}
		}
		
		// Validate crypto algorithms
		if !v.isValidEncryption(tunnel.Crypto.Encryption) {
			return fmt.Errorf("tunnel %d (%s): invalid encryption algorithm: %s", i, tunnel.Name, tunnel.Crypto.Encryption)
		}
		
		if !v.isValidIntegrity(tunnel.Crypto.Integrity) {
			return fmt.Errorf("tunnel %d (%s): invalid integrity algorithm: %s", i, tunnel.Name, tunnel.Crypto.Integrity)
		}
		
		if !v.isValidDHGroup(tunnel.Crypto.DHGroup) {
			return fmt.Errorf("tunnel %d (%s): invalid DH group: %s", i, tunnel.Name, tunnel.Crypto.DHGroup)
		}
		
		// Validate lifetime
		if tunnel.Crypto.Lifetime == 0 {
			return fmt.Errorf("tunnel %d (%s): SA lifetime must be specified", i, tunnel.Name)
		}
		if tunnel.Crypto.Lifetime < 5*time.Minute {
			return fmt.Errorf("tunnel %d (%s): SA lifetime too short (minimum 5 minutes)", i, tunnel.Name)
		}
		if tunnel.Crypto.Lifetime > 24*time.Hour {
			return fmt.Errorf("tunnel %d (%s): SA lifetime too long (maximum 24 hours)", i, tunnel.Name)
		}
	}
	
	return nil
}

func (v *SecurityValidator) isValidEncryption(alg ipsec.EncryptionAlgorithm) bool {
	validAlgs := []ipsec.EncryptionAlgorithm{
		ipsec.EncryptionAES128,
		ipsec.EncryptionAES256,
		ipsec.EncryptionAES128GCM,
		ipsec.EncryptionAES256GCM,
		ipsec.Encryption3DES,
	}
	
	for _, valid := range validAlgs {
		if alg == valid {
			return true
		}
	}
	return false
}

func (v *SecurityValidator) isValidIntegrity(alg ipsec.IntegrityAlgorithm) bool {
	validAlgs := []ipsec.IntegrityAlgorithm{
		ipsec.IntegritySHA1,
		ipsec.IntegritySHA256,
		ipsec.IntegritySHA384,
		ipsec.IntegritySHA512,
	}
	
	for _, valid := range validAlgs {
		if alg == valid {
			return true
		}
	}
	return false
}

func (v *SecurityValidator) isValidDHGroup(group ipsec.DHGroup) bool {
	validGroups := []ipsec.DHGroup{
		ipsec.DHGroupModp1024,
		ipsec.DHGroupModp1536,
		ipsec.DHGroupModp2048,
		ipsec.DHGroupModp3072,
		ipsec.DHGroupModp4096,
		ipsec.DHGroupModp8192,
		ipsec.DHGroupECP256,
		ipsec.DHGroupECP384,
		ipsec.DHGroupECP521,
	}
	
	for _, valid := range validGroups {
		if group == valid {
			return true
		}
	}
	return false
}

// PlatformCompatibilityValidator validates platform-specific constraints
type PlatformCompatibilityValidator struct{}

func (v *PlatformCompatibilityValidator) Validate(policy *Policy) error {
	// Check for platform-specific limitations
	for i, tunnel := range policy.Tunnels {
		// AH mode has limited support on some platforms
		if tunnel.Mode == ipsec.ModeAHTunnel || tunnel.Mode == ipsec.ModeAHTransport {
			// Warn but don't fail - let platform-specific managers handle it
		}
		
		// Combined ESP+AH mode
		if tunnel.Mode == ipsec.ModeESPAHTunnel {
			// Some platforms may not support this
		}
		
		// GCM modes require modern strongSwan/IPsec implementation
		if tunnel.Crypto.Encryption == ipsec.EncryptionAES128GCM || 
		   tunnel.Crypto.Encryption == ipsec.EncryptionAES256GCM {
			if tunnel.Crypto.IKEVersion != ipsec.IKEv2 {
				return fmt.Errorf("tunnel %d (%s): GCM modes require IKEv2", i, tunnel.Name)
			}
		}
	}
	
	return nil
}

// DefaultPolicy returns a default policy template
func DefaultPolicy() *Policy {
	return &Policy{
		Name:      "default-policy",
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Enabled:   true,
		Priority:  0,
		Tunnels: []ipsec.TunnelConfig{
			{
				Name:          "example-tunnel",
				Mode:          ipsec.ModeESPTunnel,
				LocalAddress:  "10.0.1.1",
				RemoteAddress: "10.0.2.1",
				Crypto: ipsec.CryptoConfig{
					Encryption: ipsec.EncryptionAES256,
					Integrity:  ipsec.IntegritySHA256,
					DHGroup:    ipsec.DHGroupModp2048,
					IKEVersion: ipsec.IKEv2,
					Lifetime:   time.Hour,
				},
				Auth: ipsec.AuthConfig{
					Type:   ipsec.AuthPSK,
					Secret: "ChangeMe123!",
				},
				TrafficSelectors: []ipsec.TrafficSelector{
					{
						LocalSubnet:  "10.0.1.0/24",
						RemoteSubnet: "10.0.2.0/24",
					},
				},
				DPD: ipsec.DPDConfig{
					Delay:  30 * time.Second,
					Action: "restart",
				},
				AutoStart: true,
			},
		},
	}
}
