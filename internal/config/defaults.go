package config

const (
	DefaultPort                = 7777
	DefaultHost                = "127.0.0.1"
	DefaultLogLevel            = "info"
	DefaultLogFormat           = "text"
	DefaultDataDir             = "~/.aetronyx"
	DefaultDBName              = "aetronyx.db"
	DefaultAuthTokenFile       = "auth-token"
	DefaultKeyFile             = "audit.ed25519"
	DefaultPlanningModel       = "anthropic/claude-opus-4-6"
	DefaultExecutionModel      = "anthropic/claude-haiku-4-5"
	DefaultMaxIterations       = 30
	DefaultMaxCostUSD          = 5.00
	DefaultMaxWallTimeMinutes  = 60
	DefaultRetentionDays       = 2555
	DefaultOtelEndpoint        = ""
	DefaultTheme               = "system"
	DefaultView                = "tree"
)

// Defaults returns a Config with sensible built-in defaults.
func Defaults() Config {
	return Config{
		Server: Server{
			Host:         DefaultHost,
			Port:         DefaultPort,
			AllowRemote:  false,
			OpenBrowser:  true,
		},
		Storage: Storage{
			DataDir:    DefaultDataDir,
			DBFilename: DefaultDBName,
		},
		Providers: Providers{
			Anthropic: Anthropic{
				APIKeyEnv: "ANTHROPIC_API_KEY",
				Timeout:   120,
			},
			OpenAI: OpenAI{
				APIKeyEnv: "OPENAI_API_KEY",
				Timeout:   120,
			},
			Ollama: Ollama{
				BaseURL: "http://localhost:11434",
			},
			Bedrock: Bedrock{
				Region: "us-east-1",
			},
			Vertex: Vertex{
				Location: "us-central1",
			},
			OpenRouter: OpenRouter{
				APIKeyEnv: "OPENROUTER_API_KEY",
			},
		},
		Defaults: Defaults_{
			PlanningModel:    DefaultPlanningModel,
			ExecutionModel:   DefaultExecutionModel,
			MaxIterations:    DefaultMaxIterations,
			MaxCostUSD:       DefaultMaxCostUSD,
			MaxWallTimeMin:   DefaultMaxWallTimeMinutes,
		},
		Approval: Approval{
			RequirePlanning:     true,
			RequireSchemaChange: true,
			RequireMerge:        false,
			DefaultDecider:      "local",
		},
		Audit: Audit{
			Enabled:        true,
			RetentionDays:  DefaultRetentionDays,
			SigningKeyPath: "~/.aetronyx/audit.ed25519",
			ExportFormat:   "otel",
		},
		Cost: Cost{
			AlertThresholds:   []float64{0.7, 0.85, 0.95},
			HardStopAtBudget:  true,
		},
		UI: UI{
			Theme:       DefaultTheme,
			DefaultView: DefaultView,
		},
		Telemetry: Telemetry{
			OtelEnabled:  false,
			OtelEndpoint: DefaultOtelEndpoint,
			OtelHeaders:  make(map[string]string),
		},
		Logging: Logging{
			Level:  DefaultLogLevel,
			Format: DefaultLogFormat,
		},
	}
}
