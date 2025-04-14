package search

import (
	"fmt"
	"os/exec"
	"strings"
)

func (f *Finder) searchFiles(pattern, directory string) []string {
	if f.Debug {
		fmt.Println("\n\033[1;36mSearch Request:\033[0m")
		fmt.Println("  Pattern:    ", pattern)
		fmt.Println("  Directory:  ", directory)
		fmt.Println("  Using:      ", map[bool]string{true: "ripgrep", false: "grep"}[f.UseRipgrep])
	}

	matches := f.executeSearchCommand(pattern, directory)

	if f.Debug {
		if len(matches) > 0 {
			fmt.Printf("\033[1;32m✓ Search complete: Found %d matches\033[0m\n", len(matches))
		} else {
			fmt.Println("\033[1;31m✗ Search complete: No matches found\033[0m")
		}
	}

	return matches
}

// executeSearchCommand performs the actual command execution
func (f *Finder) executeSearchCommand(pattern, directory string) []string {
	var cmd *exec.Cmd
	var output []byte
	var err error

	var shellCmd string
	if f.UseRipgrep {
		shellCmd = fmt.Sprintf("rg --no-heading --with-filename --line-number -- %s %s",
			pattern, directory)
	} else {
		shellCmd = fmt.Sprintf("grep -r -n -F %s %s",
			pattern, directory)
	}

	if f.Debug {
		fmt.Println("\n\033[1;36mCommand Details:\033[0m")
		fmt.Println("  Shell command: ", shellCmd)
	
	}

	cmd = exec.Command("sh", "-c", shellCmd)

	output, err = cmd.Output()
	if err != nil {
		// Exit code 1 means no matches found, which is expected
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			if f.Debug {
				fmt.Println("\033[1;33m→ No matches found (exit code 1)\033[0m")
			}
			return []string{}
		}
		if f.Debug {
			fmt.Printf("\033[1;31m✗ Error running search command: %v\033[0m\n", err)
			if exitErr, ok := err.(*exec.ExitError); ok {
				fmt.Printf("  Exit code: %d\n", exitErr.ExitCode())
				fmt.Printf("  Stderr: %s\n", string(exitErr.Stderr))
			}
		}
		return []string{}
	}

	matches := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(matches) == 1 && matches[0] == "" {
		if f.Debug {
			fmt.Println("\033[1;33m→ Empty output, no matches\033[0m")
		}
		return []string{}
	}

	if f.Debug {
		fmt.Printf("\033[1;32m✓ Found %d matches\033[0m\n", len(matches))
	}

	return matches
}

