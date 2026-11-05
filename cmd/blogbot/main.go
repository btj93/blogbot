package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/btj93/blogbot/config"
	"github.com/btj93/blogbot/observability"
)

var (
	cfgPath string

	rootCmd = &cobra.Command{
		Use:           "blogbot",
		Short:         "Telegram bot for Japanese idol group blog and Showroom notifications",
		SilenceErrors: true,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "./config.toml", "path to config file")
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	observability.InitLogging(cfg.Observability.LogLevel)

	return cfg, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", slog.String("error", observability.RedactTelegramTokens(err.Error())))
		os.Exit(1)
	}
}
