package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/swavlamban/ipsec-manager/internal/server"
)

//go:embed all:dist
var webAssets embed.FS

var (
	Version   = "dev"
	BuildTime = "unknown"
	cfgFile   string
)

func main() {
	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute command")
	}
}

var rootCmd = &cobra.Command{
	Use:   "ipsec-server",
	Short: "IPsec Server - Central policy management server",
	Long: `IPsec Server provides centralized management of IPsec policies
and monitors all connected agents.`,
	Version: Version,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer(cmd.Context())
	},
}

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage policies",
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement policy list
		fmt.Println("Policy list not yet implemented")
		return nil
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: /etc/ipsec-server/config.yaml)")
	rootCmd.PersistentFlags().String("listen", ":8080", "Listen address")
	rootCmd.PersistentFlags().String("db-path", "./data/ipsec.db", "Database path")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level")
	
	viper.BindPFlag("server.listen", rootCmd.PersistentFlags().Lookup("listen"))
	viper.BindPFlag("server.db_path", rootCmd.PersistentFlags().Lookup("db-path"))
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))

	// Add subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyListCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("/etc/ipsec-server")
		viper.AddConfigPath("$HOME/.ipsec-server")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("server.listen", ":8080")
	viper.SetDefault("server.db_path", "./data/ipsec.db")
	viper.SetDefault("log.level", "info")

	if err := viper.ReadInConfig(); err == nil {
		log.Debug().Str("config", viper.ConfigFileUsed()).Msg("Using config file")
	}

	// Setup log level
	level, err := zerolog.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}

func runServer(ctx context.Context) error {
	log.Info().
		Str("version", Version).
		Str("build_time", BuildTime).
		Msg("Starting IPsec Server")

	// Create server instance
	srv, err := server.New()
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	defer srv.Close()

	// Setup Echo
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	// Register API routes
	srv.RegisterRoutes(e)

	// Serve static web dashboard
	webFS, err := fs.Sub(webAssets, "dist")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load web assets, dashboard will not be available")
	} else {
		e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(webFS))))
	}

	// Start server
	listenAddr := viper.GetString("server.listen")
	go func() {
		log.Info().Str("address", listenAddr).Msg("Server listening")
		if err := e.Start(listenAddr); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	log.Info().Msg("Server stopped")
	return nil
}
