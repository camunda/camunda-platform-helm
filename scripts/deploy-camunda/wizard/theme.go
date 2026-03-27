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
	red := lipgloss.Color("1")
	dimGray := lipgloss.Color("8")
	lightGray := lipgloss.Color("7")

	// Focused styles — active field
	t.Focused.Title = t.Focused.Title.Foreground(cyan).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(dimGray)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(cyan)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(magenta)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(magenta)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(lightGray)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("0")).Background(cyan)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(dimGray)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(cyan).Bold(true)
	t.Focused.Next = t.Focused.Next.Foreground(cyan)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(green)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(magenta)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(dimGray)

	// Blurred styles — completed/inactive fields
	t.Blurred.Title = t.Blurred.Title.Foreground(dimGray)
	t.Blurred.Description = t.Blurred.Description.Foreground(dimGray)
	t.Blurred.SelectedOption = t.Blurred.SelectedOption.Foreground(lightGray)
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(lightGray)
	t.Blurred.TextInput.Placeholder = t.Blurred.TextInput.Placeholder.Foreground(dimGray)

	// Help styles
	t.Help.ShortKey = t.Help.ShortKey.Foreground(dimGray)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(dimGray)

	return t
}
