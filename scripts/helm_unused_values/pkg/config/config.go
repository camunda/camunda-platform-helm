// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

// Config holds all the runtime settings for the application
type Config struct {
	TemplatesDir     string
	NoColors         bool
	ShowAllKeys      bool
	JSONOutput       bool
	ExitCodeOnUnused int
	QuietMode        bool
	FilterPattern    string
	Debug            bool
	UseRipgrep       bool
	SearchTool       string // Preferred search tool (ripgrep or grep)
	Parallelism      int    // Number of parallel workers (0 = auto)
}

// New creates a new configuration with default values
func New() *Config {
	return &Config{
		ExitCodeOnUnused: 0,  // Default: Don't fail on unused values
		SearchTool:       "", // Empty means auto-detect (ripgrep if available)
		Parallelism:      0,  // Default: Auto (set based on CPU cores)
	}
}
