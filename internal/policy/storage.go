package policy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // SQLite driver
)

// Storage handles persistent storage of policies and peer information
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new storage instance
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{db: db}
	if err := storage.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return storage, nil
}

// initialize creates database tables if they don't exist
func (s *Storage) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		version INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		priority INTEGER NOT NULL DEFAULT 0,
		applies_to TEXT, -- JSON array
		tunnels TEXT NOT NULL, -- JSON array
		UNIQUE(name)
	);

	CREATE TABLE IF NOT EXISTS peers (
		id TEXT PRIMARY KEY,
		hostname TEXT NOT NULL,
		platform TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		version TEXT NOT NULL,
		tags TEXT, -- JSON array
		last_seen_at TIMESTAMP NOT NULL,
		registered_at TIMESTAMP NOT NULL,
		metadata TEXT, -- JSON object
		status TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		user_id TEXT,
		details TEXT, -- JSON object
		ip_address TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_policies_enabled ON policies(enabled);
	CREATE INDEX IF NOT EXISTS idx_policies_priority ON policies(priority DESC);
	CREATE INDEX IF NOT EXISTS idx_peers_last_seen ON peers(last_seen_at DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp DESC);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// SavePolicy saves or updates a policy
func (s *Storage) SavePolicy(ctx context.Context, policy *Policy) error {
	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	policy.UpdatedAt = time.Now()

	// Serialize tunnels and applies_to to JSON
	tunnelsJSON, err := json.Marshal(policy.Tunnels)
	if err != nil {
		return fmt.Errorf("failed to marshal tunnels: %w", err)
	}

	appliesToJSON, err := json.Marshal(policy.AppliesTo)
	if err != nil {
		return fmt.Errorf("failed to marshal applies_to: %w", err)
	}

	query := `
	INSERT INTO policies (id, name, description, version, created_at, updated_at, enabled, priority, applies_to, tunnels)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		description = excluded.description,
		version = excluded.version,
		updated_at = excluded.updated_at,
		enabled = excluded.enabled,
		priority = excluded.priority,
		applies_to = excluded.applies_to,
		tunnels = excluded.tunnels
	`

	_, err = s.db.ExecContext(ctx, query,
		policy.ID, policy.Name, policy.Description, policy.Version,
		policy.CreatedAt, policy.UpdatedAt, policy.Enabled, policy.Priority,
		string(appliesToJSON), string(tunnelsJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	return nil
}

// GetPolicy retrieves a policy by ID
func (s *Storage) GetPolicy(ctx context.Context, id string) (*Policy, error) {
	query := `
	SELECT id, name, description, version, created_at, updated_at, enabled, priority, applies_to, tunnels
	FROM policies WHERE id = ?
	`

	var policy Policy
	var appliesToJSON, tunnelsJSON string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&policy.ID, &policy.Name, &policy.Description, &policy.Version,
		&policy.CreatedAt, &policy.UpdatedAt, &policy.Enabled, &policy.Priority,
		&appliesToJSON, &tunnelsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	if err := json.Unmarshal([]byte(appliesToJSON), &policy.AppliesTo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal applies_to: %w", err)
	}

	if err := json.Unmarshal([]byte(tunnelsJSON), &policy.Tunnels); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tunnels: %w", err)
	}

	return &policy, nil
}

// ListPolicies retrieves all policies
func (s *Storage) ListPolicies(ctx context.Context, enabledOnly bool) ([]Policy, error) {
	query := `
	SELECT id, name, description, version, created_at, updated_at, enabled, priority, applies_to, tunnels
	FROM policies
	`
	
	if enabledOnly {
		query += " WHERE enabled = 1"
	}
	
	query += " ORDER BY priority DESC, name ASC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var policy Policy
		var appliesToJSON, tunnelsJSON string

		err := rows.Scan(
			&policy.ID, &policy.Name, &policy.Description, &policy.Version,
			&policy.CreatedAt, &policy.UpdatedAt, &policy.Enabled, &policy.Priority,
			&appliesToJSON, &tunnelsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}

		if err := json.Unmarshal([]byte(appliesToJSON), &policy.AppliesTo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal applies_to: %w", err)
		}

		if err := json.Unmarshal([]byte(tunnelsJSON), &policy.Tunnels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tunnels: %w", err)
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

// DeletePolicy deletes a policy by ID
func (s *Storage) DeletePolicy(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM policies WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("policy not found: %s", id)
	}

	return nil
}

// RegisterPeer registers or updates a peer
func (s *Storage) RegisterPeer(ctx context.Context, peer *PeerInfo) error {
	if peer.ID == "" {
		peer.ID = uuid.New().String()
	}
	
	if peer.RegisteredAt.IsZero() {
		peer.RegisteredAt = time.Now()
	}
	peer.LastSeenAt = time.Now()

	tagsJSON, err := json.Marshal(peer.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(peer.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
	INSERT INTO peers (id, hostname, platform, ip_address, version, tags, last_seen_at, registered_at, metadata, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		hostname = excluded.hostname,
		platform = excluded.platform,
		ip_address = excluded.ip_address,
		version = excluded.version,
		tags = excluded.tags,
		last_seen_at = excluded.last_seen_at,
		metadata = excluded.metadata,
		status = excluded.status
	`

	_, err = s.db.ExecContext(ctx, query,
		peer.ID, peer.Hostname, peer.Platform, peer.IPAddress, peer.Version,
		string(tagsJSON), peer.LastSeenAt, peer.RegisteredAt, string(metadataJSON), peer.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to register peer: %w", err)
	}

	return nil
}

// GetPeer retrieves a peer by ID
func (s *Storage) GetPeer(ctx context.Context, id string) (*PeerInfo, error) {
	query := `
	SELECT id, hostname, platform, ip_address, version, tags, last_seen_at, registered_at, metadata, status
	FROM peers WHERE id = ?
	`

	var peer PeerInfo
	var tagsJSON, metadataJSON string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&peer.ID, &peer.Hostname, &peer.Platform, &peer.IPAddress, &peer.Version,
		&tagsJSON, &peer.LastSeenAt, &peer.RegisteredAt, &metadataJSON, &peer.Status,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("peer not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	if err := json.Unmarshal([]byte(tagsJSON), &peer.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}

	if err := json.Unmarshal([]byte(metadataJSON), &peer.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &peer, nil
}

// ListPeers retrieves all peers
func (s *Storage) ListPeers(ctx context.Context) ([]PeerInfo, error) {
	query := `
	SELECT id, hostname, platform, ip_address, version, tags, last_seen_at, registered_at, metadata, status
	FROM peers ORDER BY last_seen_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list peers: %w", err)
	}
	defer rows.Close()

	var peers []PeerInfo
	for rows.Next() {
		var peer PeerInfo
		var tagsJSON, metadataJSON string

		err := rows.Scan(
			&peer.ID, &peer.Hostname, &peer.Platform, &peer.IPAddress, &peer.Version,
			&tagsJSON, &peer.LastSeenAt, &peer.RegisteredAt, &metadataJSON, &peer.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan peer: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &peer.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}

		if err := json.Unmarshal([]byte(metadataJSON), &peer.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		peers = append(peers, peer)
	}

	return peers, nil
}

// UpdatePeerStatus updates the status of a peer
func (s *Storage) UpdatePeerStatus(ctx context.Context, id string, status PeerStatus) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE peers SET status = ?, last_seen_at = ? WHERE id = ?",
		status, time.Now(), id,
	)
	return err
}

// AuditLog logs an audit event
func (s *Storage) AuditLog(ctx context.Context, action, resourceType, resourceID, userID, ipAddress string, details interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal details: %w", err)
	}

	query := `
	INSERT INTO audit_log (timestamp, action, resource_type, resource_id, user_id, details, ip_address)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		time.Now(), action, resourceType, resourceID, userID, string(detailsJSON), ipAddress,
	)

	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}
