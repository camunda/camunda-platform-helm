package config

// Config holds all the runtime settings for the application
type Config struct {
	TemplatesDir     string
	NoColors         bool
	ShowAllKeys      bool
	JSONOutput       bool
	OutputFile       string
	ExitCodeOnUnused int
	QuietMode        bool
	FilterPattern    string
	Debug            bool
	UseRipgrep       bool
	GrepTimeout      int
	SearchTool       string // Preferred search tool (ripgrep or grep)
	UseShell         bool   // Use shell execution for commands (troubleshooting)
	ShowTestCommands bool   // Show test commands for unused keys
	Parallelism      int    // Number of parallel workers (0 = auto)
}

// New creates a new configuration with default values
func New() *Config {
	return &Config{
		GrepTimeout:      5,     // Default timeout
		ExitCodeOnUnused: 0,     // Default: Don't fail on unused values
		SearchTool:       "",    // Empty means auto-detect (ripgrep if available)
		UseShell:         false, // By default, don't use shell execution
		ShowTestCommands: false, // By default, don't show test commands
		Parallelism:      0,     // Default: Auto (set based on CPU cores)
	}
}

