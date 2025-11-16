// Package ui provides the terminal user interface components for the AskAI application.
// It uses the Bubble Tea framework for building interactive terminal applications.
package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/VarunSharma3520/AskAI/internal/config"
)

// NewTextInput creates and configures a new text input component for the chat interface.
// It sets up a text input field with a placeholder, character limit, and styling
// that matches the application's theme.
//
// Returns:
//   - textinput.Model: A configured text input model ready for use in the UI
//
// Example:
//   input := NewTextInput()
//   // Use in your Bubble Tea model's Update method
func NewTextInput() textinput.Model {
	ti := textinput.New()

	// Configure input field properties
	ti.Placeholder = "Ask Anything..."  // Placeholder text when input is empty
	ti.Focus()                         // Automatically focus the input field
	ti.CharLimit = 156                 // Maximum number of characters allowed
	ti.Width = 30                      // Initial width of the input field

	// Apply styling from the application's theme
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.MainColorForeground))
	
	return ti
}
