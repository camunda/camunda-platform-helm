package wizard

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// camundaTheme returns a huh theme matching the deploy-camunda color scheme.
func camundaTheme() *huh.Theme {
	t := huh.ThemeBase()

	cyan := lipgloss.Color("6")
	magenta := lipgloss.Color("5")
	green := lipgloss.Color("2")

	// Title styling — bold cyan
	t.Focused.Title = t.Focused.Title.Foreground(cyan).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.Color("8"))

	// Selected option — magenta
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(magenta)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(magenta)

	// Text input cursor
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(green)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(magenta)

	// Blurred states — dimmed versions
	t.Blurred.Title = t.Blurred.Title.Foreground(lipgloss.Color("8"))
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(lipgloss.Color("7"))

	return t
}
