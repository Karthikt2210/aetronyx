package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Server.Port != DefaultPort {
		t.Errorf("expected port %d, got %d", DefaultPort, cfg.Server.Port)
	}
	if cfg.Server.Host != DefaultHost {
		t.Errorf("expected host %q, got %q", DefaultHost, cfg.Server.Host)
	}
	if cfg.Logging.Level != DefaultLogLevel {
		t.Errorf("expected log level %q, got %q", DefaultLogLevel, cfg.Logging.Level)
	}
	if cfg.Logging.Format != DefaultLogFormat {
		t.Errorf("expected log format %q, got %q", DefaultLogFormat, cfg.Logging.Format)
	}
}

func TestEnvOverride(t *testing.T) {
	defer func() {
		os.Unsetenv("AETRONYX_SERVER_PORT")
		os.Unsetenv("AETRONYX_SERVER_HOST")
		os.Unsetenv("AETRONYX_LOGGING_LEVEL")
	}()

	os.Setenv("AETRONYX_SERVER_PORT", "9999")
	os.Setenv("AETRONYX_SERVER_HOST", "0.0.0.0")
	os.Setenv("AETRONYX_LOGGING_LEVEL", "debug")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %q", cfg.Server.Host)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %q", cfg.Logging.Level)
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"port 0 invalid", 0, true},
		{"port 1 valid", 1, false},
		{"port 7777 valid", 7777, false},
		{"port 65535 valid", 65535, false},
		{"port 65536 invalid", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Server.Port = tt.port
			errs := Validate(cfg)

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				if tt.wantErr {
					t.Errorf("expected error, got none")
				} else {
					t.Errorf("expected no error, got %v", errs)
				}
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{"debug valid", "debug", false},
		{"info valid", "info", false},
		{"warn valid", "warn", false},
		{"error valid", "error", false},
		{"verbose invalid", "verbose", true},
		{"trace invalid", "trace", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Logging.Level = tt.level
			errs := Validate(cfg)

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				if tt.wantErr {
					t.Errorf("expected error, got none")
				} else {
					t.Errorf("expected no error, got %v", errs)
				}
			}
		})
	}
}

func TestValidateLogFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"text valid", "text", false},
		{"json valid", "json", false},
		{"pretty invalid", "pretty", true},
		{"yaml invalid", "yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Logging.Format = tt.format
			errs := Validate(cfg)

			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				if tt.wantErr {
					t.Errorf("expected error, got none")
				} else {
					t.Errorf("expected no error, got %v", errs)
				}
			}
		})
	}
}

func TestDiscoveryOrder(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer os.Chdir(origCwd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	// Create workspace config file
	workspaceDir := filepath.Join(tmpDir, ".aetronyx")
	if err := os.MkdirAll(workspaceDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	workspaceConfig := Config{
		Server: Server{Port: 8888},
		Logging: Logging{Level: "warn"},
	}
	data, _ := yaml.Marshal(workspaceConfig)
	configPath := filepath.Join(workspaceDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test: workspace config is loaded
	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != 8888 {
		t.Errorf("expected port 8888 from file, got %d", cfg.Server.Port)
	}

	// Test: env overrides file
	defer os.Unsetenv("AETRONYX_SERVER_PORT")
	os.Setenv("AETRONYX_SERVER_PORT", "9999")
	cfg, _, err = Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999 from env, got %d", cfg.Server.Port)
	}

	// Test: flags override env
	flags := map[string]string{"server.port": "7000"}
	cfg, _, err = Load(flags)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != 7000 {
		t.Errorf("expected port 7000 from flags, got %d", cfg.Server.Port)
	}
}

func TestLoadWithMissingConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	origCwd, _ := os.Getwd()
	defer os.Chdir(origCwd)
	os.Chdir(tmpDir)

	// No config files exist, should use defaults
	cfg, warnings, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != DefaultPort {
		t.Errorf("expected default port, got %d", cfg.Server.Port)
	}
	// Warnings may be empty or contain info about missing files
	_ = warnings
}

func TestTildeExpansion(t *testing.T) {
	defer os.Unsetenv("AETRONYX_STORAGE_DATA_DIR")
	defer os.Unsetenv("AETRONYX_AUDIT_SIGNING_KEY_PATH")

	os.Setenv("AETRONYX_STORAGE_DATA_DIR", "~/.aetronyx")
	os.Setenv("AETRONYX_AUDIT_SIGNING_KEY_PATH", "~/.aetronyx/key")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Storage.DataDir == "~/.aetronyx" {
		t.Error("expected tilde to be expanded in data_dir")
	}
	if !filepath.IsAbs(cfg.Storage.DataDir) {
		t.Errorf("expected absolute path, got %q", cfg.Storage.DataDir)
	}

	if cfg.Audit.SigningKeyPath == "~/.aetronyx/key" {
		t.Error("expected tilde to be expanded in signing_key_path")
	}
	if !filepath.IsAbs(cfg.Audit.SigningKeyPath) {
		t.Errorf("expected absolute path, got %q", cfg.Audit.SigningKeyPath)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	cfg := Defaults()
	cfg.Server.Port = 0       // invalid
	cfg.Logging.Level = "bad" // invalid
	cfg.Logging.Format = "bad" // invalid

	errs := Validate(cfg)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestParseLoggingLevel(t *testing.T) {
	tests := map[string]bool{
		"debug": false,
		"info":  false,
		"warn":  false,
		"error": false,
		"trace": true,
		"":      true,
	}

	for level, wantErr := range tests {
		cfg := Defaults()
		cfg.Logging.Level = level
		errs := Validate(cfg)
		hasErr := len(errs) > 0
		if hasErr != wantErr {
			if wantErr {
				t.Errorf("level %q: expected error, got none", level)
			} else {
				t.Errorf("level %q: expected no error, got %v", level, errs)
			}
		}
	}
}

func TestFlagParsing(t *testing.T) {
	tmpDir := t.TempDir()
	origCwd, _ := os.Getwd()
	defer os.Chdir(origCwd)
	os.Chdir(tmpDir)

	flags := map[string]string{
		"server.port":       "8080",
		"server.host":       "localhost",
		"logging.level":     "debug",
		"logging.format":    "json",
		"storage.data-dir":  "/tmp/data",
	}

	cfg, _, err := Load(flags)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected host localhost, got %q", cfg.Server.Host)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected level debug, got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected format json, got %q", cfg.Logging.Format)
	}
	if cfg.Storage.DataDir != "/tmp/data" {
		t.Errorf("expected data-dir /tmp/data, got %q", cfg.Storage.DataDir)
	}
}

func TestInvalidFlagValue(t *testing.T) {
	flags := map[string]string{
		"server.port": "not-a-number",
	}

	_, _, err := Load(flags)
	if err == nil {
		t.Error("expected error for invalid port value")
	}
}

func TestBoolParsing(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"true", "true", true},
		{"True", "True", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"off", "off", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBool(tt.val)
			if result != tt.want {
				t.Errorf("parseBool(%q) = %v, want %v", tt.val, result, tt.want)
			}
		})
	}
}

func TestEnvProviderConfig(t *testing.T) {
	defer func() {
		os.Unsetenv("AETRONYX_PROVIDERS_ANTHROPIC_API_KEY_ENV")
		os.Unsetenv("AETRONYX_PROVIDERS_ANTHROPIC_TIMEOUT_SECONDS")
		os.Unsetenv("AETRONYX_PROVIDERS_OPENAI_API_KEY_ENV")
		os.Unsetenv("AETRONYX_PROVIDERS_OLLAMA_BASE_URL")
	}()

	os.Setenv("AETRONYX_PROVIDERS_ANTHROPIC_API_KEY_ENV", "MY_CUSTOM_KEY")
	os.Setenv("AETRONYX_PROVIDERS_ANTHROPIC_TIMEOUT_SECONDS", "300")
	os.Setenv("AETRONYX_PROVIDERS_OPENAI_API_KEY_ENV", "OPENAI_CUSTOM")
	os.Setenv("AETRONYX_PROVIDERS_OLLAMA_BASE_URL", "http://example.com:11434")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Providers.Anthropic.APIKeyEnv != "MY_CUSTOM_KEY" {
		t.Errorf("expected Anthropic API key env MY_CUSTOM_KEY, got %q", cfg.Providers.Anthropic.APIKeyEnv)
	}
	if cfg.Providers.Anthropic.Timeout != 300 {
		t.Errorf("expected Anthropic timeout 300, got %d", cfg.Providers.Anthropic.Timeout)
	}
	if cfg.Providers.OpenAI.APIKeyEnv != "OPENAI_CUSTOM" {
		t.Errorf("expected OpenAI API key env OPENAI_CUSTOM, got %q", cfg.Providers.OpenAI.APIKeyEnv)
	}
	if cfg.Providers.Ollama.BaseURL != "http://example.com:11434" {
		t.Errorf("expected Ollama base URL http://example.com:11434, got %q", cfg.Providers.Ollama.BaseURL)
	}
}

func TestEnvDefaults(t *testing.T) {
	defer func() {
		os.Unsetenv("AETRONYX_DEFAULTS_PLANNING_MODEL")
		os.Unsetenv("AETRONYX_DEFAULTS_EXECUTION_MODEL")
		os.Unsetenv("AETRONYX_DEFAULTS_MAX_ITERATIONS")
		os.Unsetenv("AETRONYX_DEFAULTS_MAX_COST_USD")
		os.Unsetenv("AETRONYX_DEFAULTS_MAX_WALL_TIME_MINUTES")
	}()

	os.Setenv("AETRONYX_DEFAULTS_PLANNING_MODEL", "custom/model-1")
	os.Setenv("AETRONYX_DEFAULTS_EXECUTION_MODEL", "custom/model-2")
	os.Setenv("AETRONYX_DEFAULTS_MAX_ITERATIONS", "50")
	os.Setenv("AETRONYX_DEFAULTS_MAX_COST_USD", "10.50")
	os.Setenv("AETRONYX_DEFAULTS_MAX_WALL_TIME_MINUTES", "120")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Defaults.PlanningModel != "custom/model-1" {
		t.Errorf("expected planning model custom/model-1, got %q", cfg.Defaults.PlanningModel)
	}
	if cfg.Defaults.ExecutionModel != "custom/model-2" {
		t.Errorf("expected execution model custom/model-2, got %q", cfg.Defaults.ExecutionModel)
	}
	if cfg.Defaults.MaxIterations != 50 {
		t.Errorf("expected max iterations 50, got %d", cfg.Defaults.MaxIterations)
	}
	if cfg.Defaults.MaxCostUSD != 10.50 {
		t.Errorf("expected max cost 10.50, got %f", cfg.Defaults.MaxCostUSD)
	}
	if cfg.Defaults.MaxWallTimeMin != 120 {
		t.Errorf("expected max wall time 120, got %d", cfg.Defaults.MaxWallTimeMin)
	}
}

func TestEnvAudit(t *testing.T) {
	defer func() {
		os.Unsetenv("AETRONYX_AUDIT_ENABLED")
		os.Unsetenv("AETRONYX_AUDIT_RETENTION_DAYS")
		os.Unsetenv("AETRONYX_AUDIT_EXPORT_FORMAT")
	}()

	os.Setenv("AETRONYX_AUDIT_ENABLED", "false")
	os.Setenv("AETRONYX_AUDIT_RETENTION_DAYS", "5000")
	os.Setenv("AETRONYX_AUDIT_EXPORT_FORMAT", "json")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Audit.Enabled {
		t.Error("expected audit to be disabled")
	}
	if cfg.Audit.RetentionDays != 5000 {
		t.Errorf("expected retention days 5000, got %d", cfg.Audit.RetentionDays)
	}
	if cfg.Audit.ExportFormat != "json" {
		t.Errorf("expected export format json, got %q", cfg.Audit.ExportFormat)
	}
}

func TestEnvTelemetry(t *testing.T) {
	defer func() {
		os.Unsetenv("AETRONYX_TELEMETRY_OTEL_ENABLED")
		os.Unsetenv("AETRONYX_TELEMETRY_OTEL_ENDPOINT")
	}()

	os.Setenv("AETRONYX_TELEMETRY_OTEL_ENABLED", "true")
	os.Setenv("AETRONYX_TELEMETRY_OTEL_ENDPOINT", "http://collector:4317")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Telemetry.OtelEnabled {
		t.Error("expected telemetry to be enabled")
	}
	if cfg.Telemetry.OtelEndpoint != "http://collector:4317" {
		t.Errorf("expected endpoint http://collector:4317, got %q", cfg.Telemetry.OtelEndpoint)
	}
}

func TestEnvBooleanParsing(t *testing.T) {
	defer func() {
		os.Unsetenv("AETRONYX_SERVER_ALLOW_REMOTE")
		os.Unsetenv("AETRONYX_SERVER_OPEN_BROWSER")
	}()

	os.Setenv("AETRONYX_SERVER_ALLOW_REMOTE", "true")
	os.Setenv("AETRONYX_SERVER_OPEN_BROWSER", "false")

	cfg, _, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Server.AllowRemote {
		t.Error("expected allow_remote to be true")
	}
	if cfg.Server.OpenBrowser {
		t.Error("expected open_browser to be false")
	}
}

func TestValidateUITheme(t *testing.T) {
	tests := []struct {
		name    string
		theme   string
		wantErr bool
	}{
		{"light", "light", false},
		{"dark", "dark", false},
		{"system", "system", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.UI.Theme = tt.theme
			errs := Validate(cfg)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				if tt.wantErr {
					t.Errorf("expected error, got none")
				} else {
					t.Errorf("expected no error, got %v", errs)
				}
			}
		})
	}
}

func TestValidateUIView(t *testing.T) {
	tests := []struct {
		name    string
		view    string
		wantErr bool
	}{
		{"tree", "tree", false},
		{"timeline", "timeline", false},
		{"flame", "flame", false},
		{"chart", "chart", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.UI.DefaultView = tt.view
			errs := Validate(cfg)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				if tt.wantErr {
					t.Errorf("expected error, got none")
				} else {
					t.Errorf("expected no error, got %v", errs)
				}
			}
		})
	}
}
