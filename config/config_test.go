package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearEnv neutralises every BLOGBOT_* binding for the duration of a test.
// Without it, a value exported in the developer's shell silently overrides the
// file or default a test is trying to assert, so the test reports on the shell
// rather than on the code. t.Setenv restores the prior value automatically.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range envBoundKeys {
		t.Setenv("BLOGBOT_"+strings.ToUpper(strings.ReplaceAll(key, ".", "_")), "")
	}
}

func TestLoad(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[telegram]
bot_token = "test-token"
log_chat_id = "12345"

[database]
dsn = "postgres://test:test@localhost:5432/testdb?sslmode=disable"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Telegram.BotToken != "test-token" {
		t.Errorf("got bot_token=%q, want %q", cfg.Telegram.BotToken, "test-token")
	}

	if cfg.Database.DSN != "postgres://test:test@localhost:5432/testdb?sslmode=disable" {
		t.Errorf("got dsn=%q, want %q", cfg.Database.DSN, "postgres://test:test@localhost:5432/testdb?sslmode=disable")
	}

	if cfg.Scraper.StaffNames[0] != "運営スタッフ" {
		t.Errorf("got staff_names=%v, want [運営スタッフ]", cfg.Scraper.StaffNames)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[telegram]
bot_token = "file-token"
log_chat_id = "12345"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BLOGBOT_TELEGRAM_BOT_TOKEN", "env-token")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Telegram.BotToken != "env-token" {
		t.Errorf("got bot_token=%q, want %q", cfg.Telegram.BotToken, "env-token")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[telegram]
bot_token = "test"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Database.DSN != "postgres://blogbot:blogbot@localhost:5432/blogbot?sslmode=disable" {
		t.Errorf(
			"Database.DSN = %q, want %q",
			cfg.Database.DSN,
			"postgres://blogbot:blogbot@localhost:5432/blogbot?sslmode=disable",
		)
	}

	if cfg.Webhook.ListenAddr != ":8080" {
		t.Errorf("Webhook.ListenAddr = %q, want %q", cfg.Webhook.ListenAddr, ":8080")
	}

	if cfg.Observability.LogLevel != "info" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "info")
	}

	if len(cfg.Scraper.StaffNames) != 1 || cfg.Scraper.StaffNames[0] != "運営スタッフ" {
		t.Errorf("Scraper.StaffNames = %v, want [運営スタッフ]", cfg.Scraper.StaffNames)
	}
}

func TestLoad_EnvOverrideNested(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[telegram]
bot_token = "test"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BLOGBOT_DATABASE_DSN", "postgres://custom:custom@localhost:5432/custom?sslmode=disable")
	t.Setenv("BLOGBOT_OBSERVABILITY_LOG_LEVEL", "debug")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Database.DSN != "postgres://custom:custom@localhost:5432/custom?sslmode=disable" {
		t.Errorf(
			"Database.DSN = %q, want %q",
			cfg.Database.DSN,
			"postgres://custom:custom@localhost:5432/custom?sslmode=disable",
		)
	}

	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "debug")
	}
}

// A missing config file is tolerated so the process can run purely from the
// environment, but it must still fail when the environment supplies nothing:
// booting on defaults would silently point the bot at the wrong database.
// The env is cleared explicitly so an ambient BLOGBOT_* value in the
// developer's shell cannot make this pass or fail for the wrong reason.
func TestLoad_MissingFileAndEmptyEnvIsRejected(t *testing.T) {
	t.Setenv("BLOGBOT_TELEGRAM_BOT_TOKEN", "")
	t.Setenv("BLOGBOT_DATABASE_DSN", "")

	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("expected error when neither a config file nor the environment supplies credentials, got nil")
	}
}

// The mirror case: no config file at all, but the environment is complete.
func TestLoad_MissingFileWithCompleteEnvSucceeds(t *testing.T) {
	t.Setenv("BLOGBOT_TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("BLOGBOT_DATABASE_DSN", "postgres://u:p@localhost:5432/db?sslmode=disable")

	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatalf("expected env-only configuration to load, got %v", err)
	}

	if cfg.Telegram.BotToken != "test-token" {
		t.Errorf("BotToken = %q, want %q", cfg.Telegram.BotToken, "test-token")
	}
}
