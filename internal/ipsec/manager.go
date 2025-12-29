package ipsec

import (
	"context"
	"time"
)

// IPsecMode represents the IPsec operational mode
type IPsecMode string

const (
	ModeESPTunnel    IPsecMode = "esp-tunnel"
	ModeESPTransport IPsecMode = "esp-transport"
	ModeAHTunnel     IPsecMode = "ah-tunnel"
	ModeAHTransport  IPsecMode = "ah-transport"
	ModeESPAHTunnel  IPsecMode = "esp-ah-tunnel"
)

// AuthType represents authentication method
type AuthType string

const (
	AuthPSK         AuthType = "psk"
	AuthCertificate AuthType = "certificate"
)

// EncryptionAlgorithm represents encryption algorithm
type EncryptionAlgorithm string

const (
	EncryptionAES128    EncryptionAlgorithm = "aes128"
	EncryptionAES256    EncryptionAlgorithm = "aes256"
	EncryptionAES128GCM EncryptionAlgorithm = "aes128gcm"
	EncryptionAES256GCM EncryptionAlgorithm = "aes256gcm"
	Encryption3DES      EncryptionAlgorithm = "3des"
)

// IntegrityAlgorithm represents integrity/hash algorithm
type IntegrityAlgorithm string

const (
	IntegritySHA1   IntegrityAlgorithm = "sha1"
	IntegritySHA256 IntegrityAlgorithm = "sha256"
	IntegritySHA384 IntegrityAlgorithm = "sha384"
	IntegritySHA512 IntegrityAlgorithm = "sha512"
)

// DHGroup represents Diffie-Hellman group
type DHGroup string

const (
	DHGroupModp1024  DHGroup = "modp1024"
	DHGroupModp1536  DHGroup = "modp1536"
	DHGroupModp2048  DHGroup = "modp2048"
	DHGroupModp3072  DHGroup = "modp3072"
	DHGroupModp4096  DHGroup = "modp4096"
	DHGroupModp8192  DHGroup = "modp8192"
	DHGroupECP256    DHGroup = "ecp256"
	DHGroupECP384    DHGroup = "ecp384"
	DHGroupECP521    DHGroup = "ecp521"
)

// IKEVersion represents IKE protocol version
type IKEVersion string

const (
	IKEv1 IKEVersion = "ikev1"
	IKEv2 IKEVersion = "ikev2"
)

// TunnelState represents the current state of a tunnel
type TunnelState string

const (
	StateDown        TunnelState = "down"
	StateConnecting  TunnelState = "connecting"
	StateEstablished TunnelState = "established"
	StateRekeying    TunnelState = "rekeying"
	StateError       TunnelState = "error"
)

// CryptoConfig defines cryptographic parameters
type CryptoConfig struct {
	Encryption EncryptionAlgorithm `json:"encryption" yaml:"encryption"`
	Integrity  IntegrityAlgorithm  `json:"integrity" yaml:"integrity"`
	DHGroup    DHGroup             `json:"dhgroup" yaml:"dhgroup"`
	IKEVersion IKEVersion          `json:"ikeversion" yaml:"ikeversion"`
	Lifetime   time.Duration       `json:"lifetime" yaml:"lifetime"` // SA lifetime
}

// AuthConfig defines authentication configuration
type AuthConfig struct {
	Type       AuthType `json:"type" yaml:"type"`
	Secret     string   `json:"secret,omitempty" yaml:"secret,omitempty"`         // PSK
	CertPath   string   `json:"cert_path,omitempty" yaml:"cert_path,omitempty"`   // Certificate path
	KeyPath    string   `json:"key_path,omitempty" yaml:"key_path,omitempty"`     // Private key path
	CACertPath string   `json:"ca_cert_path,omitempty" yaml:"ca_cert_path,omitempty"` // CA certificate path
}

// TrafficSelector defines which traffic should be encrypted
type TrafficSelector struct {
	LocalSubnet  string   `json:"local_subnet" yaml:"local_subnet"`   // e.g., "10.0.1.0/24"
	RemoteSubnet string   `json:"remote_subnet" yaml:"remote_subnet"` // e.g., "10.0.2.0/24"
	Protocol     string   `json:"protocol,omitempty" yaml:"protocol,omitempty"` // tcp, udp, icmp, or empty for all
	LocalPort    uint16   `json:"local_port,omitempty" yaml:"local_port,omitempty"`
	RemotePort   uint16   `json:"remote_port,omitempty" yaml:"remote_port,omitempty"`
}

// DPDConfig defines Dead Peer Detection configuration
type DPDConfig struct {
	Delay  time.Duration `json:"delay" yaml:"delay"`   // How often to check
	Action string        `json:"action" yaml:"action"` // restart, clear, hold
}

// TunnelConfig defines complete tunnel configuration
type TunnelConfig struct {
	Name             string            `json:"name" yaml:"name"`
	Mode             IPsecMode         `json:"mode" yaml:"mode"`
	LocalAddress     string            `json:"local_address" yaml:"local_address"`
	RemoteAddress    string            `json:"remote_address" yaml:"remote_address"`
	LocalID          string            `json:"local_id,omitempty" yaml:"local_id,omitempty"`
	RemoteID         string            `json:"remote_id,omitempty" yaml:"remote_id,omitempty"`
	Crypto           CryptoConfig      `json:"crypto" yaml:"crypto"`
	Auth             AuthConfig        `json:"auth" yaml:"auth"`
	TrafficSelectors []TrafficSelector `json:"traffic_selectors" yaml:"traffic_selectors"`
	DPD              DPDConfig         `json:"dpd" yaml:"dpd"`
	AutoStart        bool              `json:"autostart" yaml:"autostart"`
	Mark             string            `json:"mark,omitempty" yaml:"mark,omitempty"` // For routing mark
}

// TunnelStatus represents the current status of a tunnel
type TunnelStatus struct {
	Name            string        `json:"name"`
	State           TunnelState   `json:"state"`
	LocalAddress    string        `json:"local_address"`
	RemoteAddress   string        `json:"remote_address"`
	EstablishedAt   time.Time     `json:"established_at,omitempty"`
	LastRekeyAt     time.Time     `json:"last_rekey_at,omitempty"`
	BytesIn         uint64        `json:"bytes_in"`
	BytesOut        uint64        `json:"bytes_out"`
	PacketsIn       uint64        `json:"packets_in"`
	PacketsOut      uint64        `json:"packets_out"`
	Uptime          time.Duration `json:"uptime"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	CurrentCrypto   CryptoConfig  `json:"current_crypto,omitempty"`
}

// TrafficStats represents traffic statistics for a tunnel
type TrafficStats struct {
	BytesIn    uint64    `json:"bytes_in"`
	BytesOut   uint64    `json:"bytes_out"`
	PacketsIn  uint64    `json:"packets_in"`
	PacketsOut uint64    `json:"packets_out"`
	Timestamp  time.Time `json:"timestamp"`
}

// SAInfo represents Security Association information
type SAInfo struct {
	LocalSPI  string    `json:"local_spi"`
	RemoteSPI string    `json:"remote_spi"`
	Crypto    string    `json:"crypto"`
	Integrity string    `json:"integrity"`
	DHGroup   string    `json:"dhgroup"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IPsecManager is the main interface for managing IPsec tunnels
// Platform-specific implementations will handle OS differences
type IPsecManager interface {
	// CreateTunnel creates a new IPsec tunnel with the given configuration
	CreateTunnel(ctx context.Context, config TunnelConfig) error

	// DeleteTunnel removes an existing IPsec tunnel
	DeleteTunnel(ctx context.Context, name string) error

	// UpdateTunnel updates an existing tunnel configuration
	UpdateTunnel(ctx context.Context, config TunnelConfig) error

	// StartTunnel initiates the IPsec tunnel connection
	StartTunnel(ctx context.Context, name string) error

	// StopTunnel terminates the IPsec tunnel connection
	StopTunnel(ctx context.Context, name string) error

	// GetTunnelStatus retrieves current status of a tunnel
	GetTunnelStatus(ctx context.Context, name string) (*TunnelStatus, error)

	// ListTunnels returns all configured tunnels
	ListTunnels(ctx context.Context) ([]TunnelStatus, error)

	// GetStatistics retrieves traffic statistics for a tunnel
	GetStatistics(ctx context.Context, name string) (*TrafficStats, error)

	// GetSAInfo retrieves Security Association information
	GetSAInfo(ctx context.Context, name string) ([]SAInfo, error)

	// ValidateConfig validates tunnel configuration for platform
	ValidateConfig(config TunnelConfig) error

	// Initialize performs platform-specific initialization
	Initialize(ctx context.Context) error

	// Cleanup performs platform-specific cleanup
	Cleanup(ctx context.Context) error
}

// ManagerFactory creates platform-specific IPsec managers
type ManagerFactory interface {
	// NewManager creates a new IPsec manager for the current platform
	NewManager() (IPsecManager, error)
}
