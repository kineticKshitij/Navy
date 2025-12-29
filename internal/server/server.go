package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/swavlamban/ipsec-manager/internal/policy"
)

// Server represents the IPsec management server
type Server struct {
	storage *policy.Storage
	engine  *policy.PolicyEngine
}

// New creates a new server instance
func New() (*Server, error) {
	// Get database path
	dbPath := viper.GetString("server.db_path")
	
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create storage
	storage, err := policy.NewStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create policy engine
	engine := policy.NewPolicyEngine()

	log.Info().Str("db_path", dbPath).Msg("Server initialized")

	return &Server{
		storage: storage,
		engine:  engine,
	}, nil
}

// Close closes the server and its resources
func (s *Server) Close() error {
	return s.storage.Close()
}

// RegisterRoutes registers all API routes
func (s *Server) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api")

	// Policy endpoints
	api.GET("/policies", s.handleListPolicies)
	api.POST("/policies", s.handleCreatePolicy)
	api.GET("/policies/:id", s.handleGetPolicy)
	api.PUT("/policies/:id", s.handleUpdatePolicy)
	api.DELETE("/policies/:id", s.handleDeletePolicy)

	// Peer endpoints
	api.POST("/peers/register", s.handleRegisterPeer)
	api.GET("/peers", s.handleListPeers)
	api.GET("/peers/:id", s.handleGetPeer)
	api.PUT("/peers/:id/status", s.handleUpdatePeerStatus)

	// Tunnel status endpoints
	api.GET("/tunnels", s.handleListTunnels)
	api.GET("/tunnels/:name", s.handleGetTunnel)

	// Health check
	api.GET("/health", s.handleHealth)
}

// Policy handlers

func (s *Server) handleListPolicies(c echo.Context) error {
	enabledOnly := c.QueryParam("enabled") == "true"
	peerID := c.QueryParam("peer_id")

	policies, err := s.storage.ListPolicies(c.Request().Context(), enabledOnly)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list policies")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list policies",
		})
	}

	// Filter for specific peer if requested
	if peerID != "" {
		peer, err := s.storage.GetPeer(c.Request().Context(), peerID)
		if err != nil {
			log.Error().Err(err).Str("peer_id", peerID).Msg("Failed to get peer")
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "Peer not found",
			})
		}

		policies = s.engine.FilterPoliciesForPeer(policies, peer)
	}

	return c.JSON(http.StatusOK, policies)
}

func (s *Server) handleCreatePolicy(c echo.Context) error {
	var pol policy.Policy
	if err := c.Bind(&pol); err != nil {
		log.Error().Err(err).Msg("Failed to bind policy JSON")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Invalid policy format: %v", err),
		})
	}

	// Validate policy
	if err := s.engine.Validate(&pol); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Policy validation failed: %v", err),
		})
	}

	// Save policy
	if err := s.storage.SavePolicy(c.Request().Context(), &pol); err != nil {
		log.Error().Err(err).Msg("Failed to save policy")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to save policy",
		})
	}

	// Audit log
	s.storage.AuditLog(c.Request().Context(), "create", "policy", pol.ID, "", 
		c.RealIP(), map[string]string{"name": pol.Name})

	log.Info().Str("policy_id", pol.ID).Str("name", pol.Name).Msg("Policy created")

	return c.JSON(http.StatusCreated, pol)
}

func (s *Server) handleGetPolicy(c echo.Context) error {
	id := c.Param("id")

	pol, err := s.storage.GetPolicy(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Policy not found",
		})
	}

	return c.JSON(http.StatusOK, pol)
}

func (s *Server) handleUpdatePolicy(c echo.Context) error {
	id := c.Param("id")

	var pol policy.Policy
	if err := c.Bind(&pol); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid policy format",
		})
	}

	pol.ID = id // Ensure ID matches URL

	// Validate policy
	if err := s.engine.Validate(&pol); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Policy validation failed: %v", err),
		})
	}

	// Save policy
	if err := s.storage.SavePolicy(c.Request().Context(), &pol); err != nil {
		log.Error().Err(err).Msg("Failed to update policy")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update policy",
		})
	}

	// Audit log
	s.storage.AuditLog(c.Request().Context(), "update", "policy", pol.ID, "", 
		c.RealIP(), map[string]string{"name": pol.Name})

	log.Info().Str("policy_id", pol.ID).Str("name", pol.Name).Msg("Policy updated")

	return c.JSON(http.StatusOK, pol)
}

func (s *Server) handleDeletePolicy(c echo.Context) error {
	id := c.Param("id")

	if err := s.storage.DeletePolicy(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Policy not found",
		})
	}

	// Audit log
	s.storage.AuditLog(c.Request().Context(), "delete", "policy", id, "", 
		c.RealIP(), nil)

	log.Info().Str("policy_id", id).Msg("Policy deleted")

	return c.NoContent(http.StatusNoContent)
}

// Peer handlers

func (s *Server) handleRegisterPeer(c echo.Context) error {
	var peer policy.PeerInfo
	if err := c.Bind(&peer); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid peer format",
		})
	}

	peer.Status = policy.PeerStatusOnline

	if err := s.storage.RegisterPeer(c.Request().Context(), &peer); err != nil {
		log.Error().Err(err).Msg("Failed to register peer")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to register peer",
		})
	}

	log.Info().
		Str("peer_id", peer.ID).
		Str("hostname", peer.Hostname).
		Str("platform", peer.Platform).
		Msg("Peer registered")

	return c.JSON(http.StatusCreated, peer)
}

func (s *Server) handleListPeers(c echo.Context) error {
	peers, err := s.storage.ListPeers(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list peers")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list peers",
		})
	}

	return c.JSON(http.StatusOK, peers)
}

func (s *Server) handleGetPeer(c echo.Context) error {
	id := c.Param("id")

	peer, err := s.storage.GetPeer(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Peer not found",
		})
	}

	return c.JSON(http.StatusOK, peer)
}

func (s *Server) handleUpdatePeerStatus(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Status policy.PeerStatus `json:"status"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	if err := s.storage.UpdatePeerStatus(c.Request().Context(), id, req.Status); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update peer status",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// Tunnel handlers

func (s *Server) handleListTunnels(c echo.Context) error {
	// TODO: Aggregate tunnel status from all peers
	return c.JSON(http.StatusOK, []map[string]string{})
}

func (s *Server) handleGetTunnel(c echo.Context) error {
	name := c.Param("name")
	// TODO: Get tunnel status
	return c.JSON(http.StatusOK, map[string]string{
		"name": name,
	})
}

// Health check

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": "0.1.0",
	})
}
