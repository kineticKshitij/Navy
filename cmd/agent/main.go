package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/swavlamban/ipsec-manager/internal/agent"
	"github.com/swavlamban/ipsec-manager/internal/ipsec"
)

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
	Use:   "ipsec-agent",
	Short: "IPsec Agent - Cross-platform IPsec tunnel management agent",
	Long: `IPsec Agent is a cross-platform daemon that manages IPsec tunnels
based on policies received from the central management server.`,
	Version: Version,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the agent in foreground mode",
	Long:  `Start the agent in foreground mode (useful for testing and debugging)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgent(cmd.Context())
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the agent as a system service",
	Long:  `Install the agent as a system service (systemd/Windows Service/launchd)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := viper.GetString("server.url")
		if serverURL == "" {
			return fmt.Errorf("server URL is required (use --server flag)")
		}

		svc, err := agent.NewService()
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}

		if err := svc.Install(); err != nil {
			return fmt.Errorf("failed to install service: %w", err)
		}

		log.Info().Msg("Service installed successfully")
		log.Info().Msgf("Server URL: %s", serverURL)
		log.Info().Msg("Start the service with: systemctl start ipsec-agent (Linux) or Start-Service ipsec-agent (Windows)")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the agent service",
	Long:  `Uninstall the agent service from the system`,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := agent.NewService()
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}

		if err := svc.Uninstall(); err != nil {
			return fmt.Errorf("failed to uninstall service: %w", err)
		}

		log.Info().Msg("Service uninstalled successfully")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent and tunnel status",
	Long:  `Display current agent status and list all configured tunnels`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showStatus(cmd.Context())
	},
}

var tunnelsCmd = &cobra.Command{
	Use:   "tunnels",
	Short: "Manage IPsec tunnels",
	Long:  `List, start, stop, and manage IPsec tunnels`,
}

var tunnelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tunnels",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listTunnels(cmd.Context())
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Force policy synchronization",
	Long:  `Force an immediate policy sync from the server`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info().Msg("Forcing policy sync...")
		// TODO: Implement sync trigger
		return fmt.Errorf("not implemented yet")
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: /etc/ipsec-agent/config.yaml)")
	rootCmd.PersistentFlags().String("server", "", "Policy server URL (e.g., https://server:8443)")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("peer-id", "", "Peer ID (auto-generated if not specified)")
	
	viper.BindPFlag("server.url", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("peer.id", rootCmd.PersistentFlags().Lookup("peer-id"))

	// Add subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(tunnelsCmd)
	
	tunnelsCmd.AddCommand(tunnelsListCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Look for config in default locations
		viper.AddConfigPath("/etc/ipsec-agent")
		viper.AddConfigPath("$HOME/.ipsec-agent")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("agent.sync_interval", "60s")
	viper.SetDefault("agent.health_check_interval", "10s")
	viper.SetDefault("server.timeout", "30s")
	viper.SetDefault("server.tls_verify", true)

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

func runAgent(ctx context.Context) error {
	log.Info().
		Str("version", Version).
		Str("build_time", BuildTime).
		Str("platform", ipsec.GetPlatform()).
		Msg("Starting IPsec Agent")

	// Check platform support
	if !ipsec.IsPlatformSupported() {
		return fmt.Errorf("unsupported platform: %s", ipsec.GetPlatform())
	}

	// Create IPsec manager
	mgr, err := ipsec.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create IPsec manager: %w", err)
	}

	if err := mgr.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize IPsec manager: %w", err)
	}
	defer mgr.Cleanup(ctx)

	// Create and start agent
	ag, err := agent.New(mgr)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	if err := ag.Start(ctx); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Wait for shutdown signal
	<-ctx.Done()

	log.Info().Msg("Shutting down agent...")
	if err := ag.Stop(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	return nil
}

func showStatus(ctx context.Context) error {
	mgr, err := ipsec.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create IPsec manager: %w", err)
	}

	if err := mgr.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize IPsec manager: %w", err)
	}
	defer mgr.Cleanup(ctx)

	tunnels, err := mgr.ListTunnels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}

	fmt.Println("IPsec Agent Status")
	fmt.Println("==================")
	fmt.Printf("Version:  %s\n", Version)
	fmt.Printf("Platform: %s\n", ipsec.GetPlatform())
	fmt.Printf("Tunnels:  %d\n\n", len(tunnels))

	if len(tunnels) == 0 {
		fmt.Println("No tunnels configured")
		return nil
	}

	fmt.Println("Tunnel Status:")
	fmt.Println("--------------")
	for _, tunnel := range tunnels {
		fmt.Printf("  %-20s  State: %-12s  In: %d bytes  Out: %d bytes\n",
			tunnel.Name, tunnel.State, tunnel.BytesIn, tunnel.BytesOut)
	}

	return nil
}

func listTunnels(ctx context.Context) error {
	return showStatus(ctx)
}
