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

package logging

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/jwalton/gchalk"
	"github.com/rs/zerolog"
	"golang.org/x/term"
)

// Logger is the global logger configured by Setup.
var Logger zerolog.Logger

// ColorEnabled controls whether chalk styling is emitted.
var ColorEnabled = true

// Initialized indicates whether Setup has been called successfully.
var Initialized = false

type Options struct {
	UseJSON      bool
	LevelString  string
	ColorEnabled bool
	// Writer overrides the default output destination (os.Stdout).
	// Set to os.Stderr when stdout must remain clean for machine-readable output.
	Writer io.Writer
}

func Setup(opts Options) error {
	level, err := parseLevel(opts.LevelString)
	if err != nil {
		level = zerolog.InfoLevel
	}
	ColorEnabled = opts.ColorEnabled && !opts.UseJSON
	// Respect standard color env vars and CI environments (e.g., GitHub Actions supports ANSI colors).
	if !opts.UseJSON {
		// Highest precedence: NO_COLOR disables color
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			ColorEnabled = false
		}
		// FORCE_COLOR enables color even when stdout is not a TTY
		if v, ok := os.LookupEnv("FORCE_COLOR"); ok && strings.TrimSpace(strings.ToLower(v)) != "0" && strings.TrimSpace(strings.ToLower(v)) != "false" {
			ColorEnabled = true
		}
		// Common CI envs that support ANSI colors (GitHub Actions, generic CI)
		if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" {
			ColorEnabled = true
		}
	}

	var w io.Writer = os.Stdout
	if opts.Writer != nil {
		w = opts.Writer
	}
	if opts.UseJSON {
		zerolog.TimeFieldFormat = time.RFC3339
		Logger = zerolog.New(w).Level(level).With().Timestamp().Logger()
		Initialized = true
		return nil
	}

	cw := zerolog.ConsoleWriter{
		Out:        w,
		NoColor:    !ColorEnabled,
		TimeFormat: "15:04:05",
		FormatLevel: func(i interface{}) string {
			lvl := strings.ToUpper(toString(i))
			if !ColorEnabled {
				return lvl
			}
			switch lvl {
			case "TRACE":
				return gchalk.Cyan(lvl)
			case "DEBUG":
				return gchalk.Blue(lvl)
			case "INFO":
				return gchalk.Green(lvl)
			case "WARN":
				return gchalk.Yellow(lvl)
			case "ERROR":
				return gchalk.Red(lvl)
			default:
				return lvl
			}
		},
	}
	Logger = zerolog.New(cw).Level(level).With().Timestamp().Logger()
	Initialized = true
	return nil
}

func parseLevel(s string) (zerolog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return zerolog.TraceLevel, nil
	case "debug":
		return zerolog.DebugLevel, nil
	case "info", "":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.InfoLevel, nil
	}
}

// ShouldLog returns true when messageLevel is at or above configuredLevel.
// Both arguments are level strings (trace, debug, info, warn, error).
func ShouldLog(configuredLevel, messageLevel string) bool {
	cfg, _ := parseLevel(configuredLevel)
	msg, _ := parseLevel(messageLevel)
	return msg >= cfg
}

// Styler is a function that styles a string (compatible with gchalk color funcs).
type Styler func(...string) string

// Emphasize wraps a string with a style when color is enabled.
func Emphasize(s string, style Styler) string {
	if !ColorEnabled {
		return s
	}
	if style == nil {
		return s
	}
	return style(s)
}

func toString(i interface{}) string {
	if i == nil {
		return ""
	}
	switch v := i.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

func CommandPrefix(cmd string) string {
	if strings.TrimSpace(cmd) == "" {
		return ""
	}
	txt := "cmd:" + cmd
	if ColorEnabled {
		txt = gchalk.Magenta(txt)
	}
	return "[" + txt + "] "
}
