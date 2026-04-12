package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load discovers and merges configuration from multiple sources.
// Priority (highest to lowest): flags → env → ./.aetronyx/config.yaml → ~/.aetronyx/config.yaml → defaults
// Returns the merged config, a list of warnings, and any error.
func Load(flags map[string]string) (Config, []string, error) {
	var warnings []string
	cfg := Defaults()

	// Load from ~/.aetronyx/config.yaml
	home, err := os.UserHomeDir()
	if err == nil {
		homeConfigPath := filepath.Join(home, ".aetronyx", "config.yaml")
		if data, err := os.ReadFile(homeConfigPath); err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to parse ~/.aetronyx/config.yaml: %v", err))
			}
		}
		// Missing home config is not an error, just use defaults
	}

	// Load from ./.aetronyx/config.yaml (workspace local)
	workspaceConfigPath := filepath.Join(".", ".aetronyx", "config.yaml")
	if data, err := os.ReadFile(workspaceConfigPath); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to parse ./.aetronyx/config.yaml: %v", err))
		}
	}
	// Missing workspace config is not an error

	// Fill in defaults for any zero-valued fields after loading from files
	fillDefaults(&cfg)

	// Load from environment variables with AETRONYX_ prefix
	mergeEnv(&cfg)

	// Apply flags (highest priority)
	if err := mergeFlags(&cfg, flags); err != nil {
		return Config{}, warnings, fmt.Errorf("invalid flag: %w", err)
	}

	// Expand tildes in all path fields
	expandTildes(&cfg)

	// Create ~/.aetronyx/ directory with mode 0700 if missing
	if err := ensureDataDir(cfg.Storage.DataDir); err != nil {
		return Config{}, warnings, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Validate configuration
	if errs := Validate(cfg); len(errs) > 0 {
		var errStrs []string
		for _, e := range errs {
			errStrs = append(errStrs, e.Error())
		}
		return Config{}, warnings, fmt.Errorf("configuration validation failed: %s", strings.Join(errStrs, "; "))
	}

	return cfg, warnings, nil
}

// fillDefaults fills in zero-valued fields with their defaults.
func fillDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = DefaultHost
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = DefaultPort
	}
	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = DefaultDataDir
	}
	if cfg.Storage.DBFilename == "" {
		cfg.Storage.DBFilename = DefaultDBName
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = DefaultLogLevel
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = DefaultLogFormat
	}
	if cfg.Providers.Anthropic.APIKeyEnv == "" {
		cfg.Providers.Anthropic.APIKeyEnv = "ANTHROPIC_API_KEY"
	}
	if cfg.Providers.Anthropic.Timeout == 0 {
		cfg.Providers.Anthropic.Timeout = 120
	}
	if cfg.Providers.OpenAI.APIKeyEnv == "" {
		cfg.Providers.OpenAI.APIKeyEnv = "OPENAI_API_KEY"
	}
	if cfg.Providers.OpenAI.Timeout == 0 {
		cfg.Providers.OpenAI.Timeout = 120
	}
	if cfg.Providers.Ollama.BaseURL == "" {
		cfg.Providers.Ollama.BaseURL = "http://localhost:11434"
	}
	if cfg.Providers.Bedrock.Region == "" {
		cfg.Providers.Bedrock.Region = "us-east-1"
	}
	if cfg.Providers.Vertex.Location == "" {
		cfg.Providers.Vertex.Location = "us-central1"
	}
	if cfg.Providers.OpenRouter.APIKeyEnv == "" {
		cfg.Providers.OpenRouter.APIKeyEnv = "OPENROUTER_API_KEY"
	}
	if cfg.Defaults.PlanningModel == "" {
		cfg.Defaults.PlanningModel = DefaultPlanningModel
	}
	if cfg.Defaults.ExecutionModel == "" {
		cfg.Defaults.ExecutionModel = DefaultExecutionModel
	}
	if cfg.Defaults.MaxIterations == 0 {
		cfg.Defaults.MaxIterations = DefaultMaxIterations
	}
	if cfg.Defaults.MaxCostUSD == 0 {
		cfg.Defaults.MaxCostUSD = DefaultMaxCostUSD
	}
	if cfg.Defaults.MaxWallTimeMin == 0 {
		cfg.Defaults.MaxWallTimeMin = DefaultMaxWallTimeMinutes
	}
	if cfg.Audit.RetentionDays == 0 {
		cfg.Audit.RetentionDays = DefaultRetentionDays
	}
	if cfg.Audit.SigningKeyPath == "" {
		cfg.Audit.SigningKeyPath = "~/.aetronyx/audit.ed25519"
	}
	if cfg.Audit.ExportFormat == "" {
		cfg.Audit.ExportFormat = "otel"
	}
	if !cfg.Audit.Enabled {
		cfg.Audit.Enabled = true
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = DefaultTheme
	}
	if cfg.UI.DefaultView == "" {
		cfg.UI.DefaultView = DefaultView
	}
	if cfg.Telemetry.OtelHeaders == nil {
		cfg.Telemetry.OtelHeaders = make(map[string]string)
	}
}

// mergeEnv applies environment variables to the config.
func mergeEnv(cfg *Config) {
	// Server
	if v := os.Getenv("AETRONYX_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("AETRONYX_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("AETRONYX_SERVER_ALLOW_REMOTE"); v != "" {
		cfg.Server.AllowRemote = parseBool(v)
	}
	if v := os.Getenv("AETRONYX_SERVER_OPEN_BROWSER"); v != "" {
		cfg.Server.OpenBrowser = parseBool(v)
	}

	// Storage
	if v := os.Getenv("AETRONYX_STORAGE_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}
	if v := os.Getenv("AETRONYX_STORAGE_DB_FILENAME"); v != "" {
		cfg.Storage.DBFilename = v
	}

	// Logging
	if v := os.Getenv("AETRONYX_LOGGING_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("AETRONYX_LOGGING_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}

	// Providers
	if v := os.Getenv("AETRONYX_PROVIDERS_ANTHROPIC_API_KEY_ENV"); v != "" {
		cfg.Providers.Anthropic.APIKeyEnv = v
	}
	if v := os.Getenv("AETRONYX_PROVIDERS_ANTHROPIC_BASE_URL"); v != "" {
		cfg.Providers.Anthropic.BaseURL = v
	}
	if v := os.Getenv("AETRONYX_PROVIDERS_ANTHROPIC_TIMEOUT_SECONDS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			cfg.Providers.Anthropic.Timeout = t
		}
	}

	if v := os.Getenv("AETRONYX_PROVIDERS_OPENAI_API_KEY_ENV"); v != "" {
		cfg.Providers.OpenAI.APIKeyEnv = v
	}
	if v := os.Getenv("AETRONYX_PROVIDERS_OPENAI_BASE_URL"); v != "" {
		cfg.Providers.OpenAI.BaseURL = v
	}
	if v := os.Getenv("AETRONYX_PROVIDERS_OPENAI_TIMEOUT_SECONDS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			cfg.Providers.OpenAI.Timeout = t
		}
	}

	if v := os.Getenv("AETRONYX_PROVIDERS_OLLAMA_BASE_URL"); v != "" {
		cfg.Providers.Ollama.BaseURL = v
	}

	// Telemetry
	if v := os.Getenv("AETRONYX_TELEMETRY_OTEL_ENABLED"); v != "" {
		cfg.Telemetry.OtelEnabled = parseBool(v)
	}
	if v := os.Getenv("AETRONYX_TELEMETRY_OTEL_ENDPOINT"); v != "" {
		cfg.Telemetry.OtelEndpoint = v
	}

	// Defaults
	if v := os.Getenv("AETRONYX_DEFAULTS_PLANNING_MODEL"); v != "" {
		cfg.Defaults.PlanningModel = v
	}
	if v := os.Getenv("AETRONYX_DEFAULTS_EXECUTION_MODEL"); v != "" {
		cfg.Defaults.ExecutionModel = v
	}
	if v := os.Getenv("AETRONYX_DEFAULTS_MAX_ITERATIONS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Defaults.MaxIterations = i
		}
	}
	if v := os.Getenv("AETRONYX_DEFAULTS_MAX_COST_USD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Defaults.MaxCostUSD = f
		}
	}
	if v := os.Getenv("AETRONYX_DEFAULTS_MAX_WALL_TIME_MINUTES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Defaults.MaxWallTimeMin = i
		}
	}

	// Audit
	if v := os.Getenv("AETRONYX_AUDIT_ENABLED"); v != "" {
		cfg.Audit.Enabled = parseBool(v)
	}
	if v := os.Getenv("AETRONYX_AUDIT_RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			cfg.Audit.RetentionDays = d
		}
	}
	if v := os.Getenv("AETRONYX_AUDIT_SIGNING_KEY_PATH"); v != "" {
		cfg.Audit.SigningKeyPath = v
	}
	if v := os.Getenv("AETRONYX_AUDIT_EXPORT_FORMAT"); v != "" {
		cfg.Audit.ExportFormat = v
	}

	// UI
	if v := os.Getenv("AETRONYX_UI_THEME"); v != "" {
		cfg.UI.Theme = v
	}
	if v := os.Getenv("AETRONYX_UI_DEFAULT_VIEW"); v != "" {
		cfg.UI.DefaultView = v
	}
}

// mergeFlags applies command-line flags to the config.
func mergeFlags(cfg *Config, flags map[string]string) error {
	for key, value := range flags {
		switch key {
		case "server.host":
			cfg.Server.Host = value
		case "server.port":
			p, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid server.port: %w", err)
			}
			cfg.Server.Port = p
		case "server.allow-remote":
			cfg.Server.AllowRemote = parseBool(value)
		case "logging.level":
			cfg.Logging.Level = value
		case "logging.format":
			cfg.Logging.Format = value
		case "storage.data-dir":
			cfg.Storage.DataDir = value
		default:
			// Ignore unknown flags
		}
	}
	return nil
}

// expandTildes expands ~ to the user's home directory in all path fields.
func expandTildes(cfg *Config) {
	home, err := os.UserHomeDir()
	if err != nil {
		return // If we can't get home dir, don't try to expand
	}

	if strings.HasPrefix(cfg.Storage.DataDir, "~") {
		cfg.Storage.DataDir = filepath.Join(home, cfg.Storage.DataDir[1:])
	}
	if strings.HasPrefix(cfg.Audit.SigningKeyPath, "~") {
		cfg.Audit.SigningKeyPath = filepath.Join(home, cfg.Audit.SigningKeyPath[1:])
	}
}

// ensureDataDir creates the data directory with mode 0700 if it doesn't exist.
func ensureDataDir(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return nil
}

// parseBool parses common boolean string representations.
func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
