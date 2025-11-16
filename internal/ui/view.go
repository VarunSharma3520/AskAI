package ui

import (
	"fmt"
	"strings"

	"github.com/VarunSharma3520/AskAI/internal/config"
	"github.com/VarunSharma3520/AskAI/internal/types"
	"github.com/charmbracelet/lipgloss"
)

// messageStyle is the style for chat messages
var messageStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("15")). // White text
	MarginLeft(1).
	MarginRight(1).
	Padding(1, 2).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Width(80) // Limit message width for better readability

// renderOptions renders the options screen with a list of selectable options
func (m Model) renderOptions() string {
	// Start building the options display
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Options"))
	sb.WriteString("\n\n")

	// Show input field if editing
	switch {
	case m.EditingModel:
		sb.WriteString("Enter model name (press Enter to save, Esc to cancel):\n")
		sb.WriteString(m.ModelInput.View())
		return sb.String()

	case m.EditingAPIURL:
		sb.WriteString("Enter API URL (press Enter to save, Esc to cancel):\n")
		sb.WriteString(m.APIURLInput.View())
		return sb.String()
	}

	// Show options list with current values
	for i, option := range m.Options {
		var optionText string
		var valueText string

		// Add current value for relevant options
		switch i {
		case 0: // Model name
			valueText = fmt.Sprintf(" (Current: %s)", m.ModelName)
		case 2: // API URL
			valueText = fmt.Sprintf(" (Current: %s)", config.APIURL())
		}

		// Format the option text with selection indicator
		if i == m.SelectedOpt {
			optionText = fmt.Sprintf("➜ %s%s", option, valueText)
		} else {
			optionText = fmt.Sprintf("  %s%s", option, valueText)
		}

		sb.WriteString(optionStyle.Render(optionText))
		sb.WriteString("\n")
	}

	return sb.String()
}

// View renders the current state of the UI based on the current screen mode
func (m Model) View() string {
	var content string
	var instructions string

	switch m.ScreenMode {
	case types.ModeChat:
		// Show the message content if it exists
		if m.Msg != "" {
			// Format the message with a nice border and padding
			msgContent := messageStyle.Render(m.Msg)
			// Add some vertical space before the input
			content = fmt.Sprintf("\n%s\n\n\n%s", msgContent, m.TextInput.View())
		} else {
			content = m.TextInput.View()
		}

		// Set instructions based on streaming state
		if m.Streaming {
			instructions = helpStyle.Render("Streaming… Press Esc to cancel, Ctrl+W to quit. Ctrl+O=Options.")
		} else {
			instructions = helpStyle.Render("Press Enter to send. Esc: Cancel, Ctrl+O: Options, Ctrl+W: Quit")
		}

	case types.ModeOptions:
		content = m.renderOptions()
		instructions = helpStyle.Render("Tab: Navigate • Enter: Select • ↑/↓: Adjust Temp • Ctrl+C: Back to Chat • Ctrl+W: Quit")

	default:
		content = "[Unknown Screen]"
	}

	// Show status message if available
	statusBar := ""
	if m.StatusMsg != "" {
		statusBar = fmt.Sprintf("\n\n%s", statusStyle.Render(m.StatusMsg))
	}

	// Combine all components with proper spacing
	return fmt.Sprintf("%s\n\n%s\n\n%s%s\n",
		titleStyle.Render("AskAI"),
		content,
		instructions,
		statusBar,
	)
}
