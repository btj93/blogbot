package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Telegram      TelegramConfig
	Database      DatabaseConfig
	Webhook       WebhookConfig
	Scraper       ScraperConfig
	Observability ObservabilityConfig
}

type ObservabilityConfig struct {
	LogLevel string `mapstructure:"log_level"`
}

type TelegramConfig struct {
	BotToken  string `mapstructure:"bot_token"`
	LogChatID string `mapstructure:"log_chat_id"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type WebhookConfig struct {
	ListenAddr string `mapstructure:"listen_addr"`
	WebhookURL string `mapstructure:"webhook_url"`
	TLSCert    string `mapstructure:"tls_cert"`
	TLSKey     string `mapstructure:"tls_key"`
	WebAppURL  string `mapstructure:"webapp_url"`
	WebAppDir  string `mapstructure:"webapp_dir"`
	UseWebApp  bool   `mapstructure:"use_webapp"`
}

type ScraperConfig struct {
	UserAgent  string   `mapstructure:"user_agent"`
	StaffNames []string `mapstructure:"staff_names"`
	LogOnly    bool     `mapstructure:"log_only"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("BLOGBOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("database.dsn", "postgres://blogbot:blogbot@localhost:5432/blogbot?sslmode=disable")
	v.SetDefault("webhook.listen_addr", ":8080")
	v.SetDefault(
		"scraper.user_agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.97 Safari/537.36",
	)
	v.SetDefault("scraper.staff_names", []string{"運営スタッフ"})
	v.SetDefault("observability.log_level", "info")

	// Bind every secret-bearing key explicitly. viper's AutomaticEnv only reaches
	// Unmarshal for keys it already knows about, so a key with no default and no
	// binding is silently ignored when supplied purely through the environment.
	for _, key := range envBoundKeys {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("binding %s: %w", key, err)
		}
	}

	// A missing config file is allowed only when the environment supplies the
	// required values; otherwise the process would boot on defaults and quietly
	// talk to the wrong database. Any other read error is always fatal.
	if err := v.ReadInConfig(); err != nil {
		var missing viper.ConfigFileNotFoundError
		if !errors.As(err, &missing) && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// envBoundKeys are the settings that carry credentials or deployment-specific
// values. Each maps to BLOGBOT_<SECTION>_<NAME>, e.g. BLOGBOT_TELEGRAM_BOT_TOKEN.
var envBoundKeys = []string{
	"telegram.bot_token",
	"telegram.log_chat_id",
	"database.dsn",
	"webhook.listen_addr",
	"webhook.webhook_url",
	"webhook.tls_cert",
	"webhook.tls_key",
	"webhook.webapp_url",
	"webhook.webapp_dir",
	"observability.log_level",
}

// validate refuses a configuration that is missing a credential rather than
// letting the process start and fail later against a default it never intended.
func (c *Config) validate() error {
	var missing []string
	if c.Telegram.BotToken == "" {
		missing = append(missing, "telegram.bot_token (BLOGBOT_TELEGRAM_BOT_TOKEN)")
	}

	if c.Database.DSN == "" {
		missing = append(missing, "database.dsn (BLOGBOT_DATABASE_DSN)")
	}

	if len(missing) > 0 {
		return fmt.Errorf("incomplete configuration: missing %s", strings.Join(missing, ", "))
	}

	return nil
}
