package config

import (
	"fmt"
	"slices"
)

// Config represents the complete Aetronyx configuration.
type Config struct {
	Server    Server      `yaml:"server" json:"server" env-prefix:"SERVER_"`
	Storage   Storage     `yaml:"storage" json:"storage" env-prefix:"STORAGE_"`
	Providers Providers   `yaml:"providers" json:"providers" env-prefix:"PROVIDERS_"`
	Defaults  Defaults_   `yaml:"defaults" json:"defaults" env-prefix:"DEFAULTS_"`
	Approval  Approval    `yaml:"approval" json:"approval" env-prefix:"APPROVAL_"`
	Audit     Audit       `yaml:"audit" json:"audit" env-prefix:"AUDIT_"`
	Cost      Cost        `yaml:"cost" json:"cost" env-prefix:"COST_"`
	UI        UI          `yaml:"ui" json:"ui" env-prefix:"UI_"`
	Telemetry Telemetry   `yaml:"telemetry" json:"telemetry" env-prefix:"TELEMETRY_"`
	Logging   Logging     `yaml:"logging" json:"logging" env-prefix:"LOGGING_"`
}

// Server contains HTTP server configuration.
type Server struct {
	Host        string `yaml:"host" json:"host" env:"HOST"`
	Port        int    `yaml:"port" json:"port" env:"PORT"`
	AllowRemote bool   `yaml:"allow_remote" json:"allow_remote" env:"ALLOW_REMOTE"`
	OpenBrowser bool   `yaml:"open_browser" json:"open_browser" env:"OPEN_BROWSER"`
}

// Storage contains database storage configuration.
type Storage struct {
	DataDir    string `yaml:"data_dir" json:"data_dir" env:"DATA_DIR"`
	DBFilename string `yaml:"db_filename" json:"db_filename" env:"DB_FILENAME"`
}

// Providers contains LLM provider configurations.
type Providers struct {
	Anthropic  Anthropic  `yaml:"anthropic" json:"anthropic"`
	OpenAI     OpenAI     `yaml:"openai" json:"openai"`
	Ollama     Ollama     `yaml:"ollama" json:"ollama"`
	Bedrock    Bedrock    `yaml:"bedrock" json:"bedrock"`
	Vertex     Vertex     `yaml:"vertex" json:"vertex"`
	OpenRouter OpenRouter `yaml:"openrouter" json:"openrouter"`
}

// Anthropic contains Anthropic provider config.
type Anthropic struct {
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env" env:"API_KEY_ENV"`
	BaseURL   string `yaml:"base_url" json:"base_url" env:"BASE_URL"`
	Timeout   int    `yaml:"timeout_seconds" json:"timeout_seconds" env:"TIMEOUT_SECONDS"`
}

// OpenAI contains OpenAI provider config.
type OpenAI struct {
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env" env:"API_KEY_ENV"`
	BaseURL   string `yaml:"base_url" json:"base_url" env:"BASE_URL"`
	Timeout   int    `yaml:"timeout_seconds" json:"timeout_seconds" env:"TIMEOUT_SECONDS"`
}

// Ollama contains Ollama provider config.
type Ollama struct {
	BaseURL string `yaml:"base_url" json:"base_url" env:"BASE_URL"`
}

// Bedrock contains AWS Bedrock provider config.
type Bedrock struct {
	Region  string `yaml:"region" json:"region" env:"REGION"`
	Profile string `yaml:"profile" json:"profile" env:"PROFILE"`
}

// Vertex contains Google Vertex AI provider config.
type Vertex struct {
	Project  string `yaml:"project" json:"project" env:"PROJECT"`
	Location string `yaml:"location" json:"location" env:"LOCATION"`
}

// OpenRouter contains OpenRouter provider config.
type OpenRouter struct {
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env" env:"API_KEY_ENV"`
}

// Defaults_ contains default settings for runs.
type Defaults_ struct {
	PlanningModel   string  `yaml:"planning_model" json:"planning_model" env:"PLANNING_MODEL"`
	ExecutionModel  string  `yaml:"execution_model" json:"execution_model" env:"EXECUTION_MODEL"`
	MaxIterations   int     `yaml:"max_iterations" json:"max_iterations" env:"MAX_ITERATIONS"`
	MaxCostUSD      float64 `yaml:"max_cost_usd" json:"max_cost_usd" env:"MAX_COST_USD"`
	MaxWallTimeMin  int     `yaml:"max_wall_time_minutes" json:"max_wall_time_minutes" env:"MAX_WALL_TIME_MINUTES"`
}

// Approval contains approval gate configuration.
type Approval struct {
	RequirePlanning     bool   `yaml:"require_planning" json:"require_planning" env:"REQUIRE_PLANNING"`
	RequireSchemaChange bool   `yaml:"require_schema_change" json:"require_schema_change" env:"REQUIRE_SCHEMA_CHANGE"`
	RequireMerge        bool   `yaml:"require_merge" json:"require_merge" env:"REQUIRE_MERGE"`
	DefaultDecider      string `yaml:"default_decider" json:"default_decider" env:"DEFAULT_DECIDER"`
}

// Audit contains audit logging configuration.
type Audit struct {
	Enabled        bool   `yaml:"enabled" json:"enabled" env:"ENABLED"`
	RetentionDays  int    `yaml:"retention_days" json:"retention_days" env:"RETENTION_DAYS"`
	SigningKeyPath string `yaml:"signing_key_path" json:"signing_key_path" env:"SIGNING_KEY_PATH"`
	ExportFormat   string `yaml:"export_format" json:"export_format" env:"EXPORT_FORMAT"`
}

// Cost contains cost tracking configuration.
type Cost struct {
	AlertThresholds  []float64 `yaml:"alert_thresholds" json:"alert_thresholds" env:"ALERT_THRESHOLDS"`
	HardStopAtBudget bool      `yaml:"hard_stop_at_budget" json:"hard_stop_at_budget" env:"HARD_STOP_AT_BUDGET"`
}

// UI contains UI/dashboard configuration.
type UI struct {
	Theme       string `yaml:"theme" json:"theme" env:"THEME"`
	DefaultView string `yaml:"default_view" json:"default_view" env:"DEFAULT_VIEW"`
}

// Telemetry contains OpenTelemetry configuration.
type Telemetry struct {
	OtelEnabled  bool              `yaml:"otel_enabled" json:"otel_enabled" env:"OTEL_ENABLED"`
	OtelEndpoint string            `yaml:"otel_endpoint" json:"otel_endpoint" env:"OTEL_ENDPOINT"`
	OtelHeaders  map[string]string `yaml:"otel_headers" json:"otel_headers"`
}

// Logging contains logging configuration.
type Logging struct {
	Level  string `yaml:"level" json:"level" env:"LEVEL"`
	Format string `yaml:"format" json:"format" env:"FORMAT"`
}

// Validate checks that the configuration values are valid.
// Returns a slice of validation errors.
func Validate(c Config) []error {
	var errs []error

	// Validate port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port))
	}

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !slices.Contains(validLevels, c.Logging.Level) {
		errs = append(errs, fmt.Errorf("logging.level must be one of %v, got %q", validLevels, c.Logging.Level))
	}

	// Validate log format
	validFormats := []string{"text", "json"}
	if !slices.Contains(validFormats, c.Logging.Format) {
		errs = append(errs, fmt.Errorf("logging.format must be one of %v, got %q", validFormats, c.Logging.Format))
	}

	// Validate theme
	validThemes := []string{"light", "dark", "system"}
	if !slices.Contains(validThemes, c.UI.Theme) {
		errs = append(errs, fmt.Errorf("ui.theme must be one of %v, got %q", validThemes, c.UI.Theme))
	}

	// Validate default view
	validViews := []string{"tree", "timeline", "flame"}
	if !slices.Contains(validViews, c.UI.DefaultView) {
		errs = append(errs, fmt.Errorf("ui.default_view must be one of %v, got %q", validViews, c.UI.DefaultView))
	}

	return errs
}
