// Package ui provides the terminal user interface components for the AskAI application.
// This file handles the update loop and message handling for the Bubble Tea TUI.
package ui

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/VarunSharma3520/AskAI/internal/config"
	"github.com/VarunSharma3520/AskAI/internal/llm"
	"github.com/VarunSharma3520/AskAI/internal/types"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// min returns the smaller of x or y
func min(x, y float64) float64 {
	return math.Min(x, y)
}

// max returns the larger of x or y
func max(x, y float64) float64 {
	return math.Max(x, y)
}

// Init initializes the TUI model with default commands and sets up the initial state.
// It's part of the Bubble Tea framework's model interface.
//
// Returns:
//   - tea.Cmd: A command that makes the text input cursor blink.
//
// Example:
//
//	program := tea.NewProgram(model)
//	// The Init method will be called automatically by Bubble Tea
func (m Model) Init() tea.Cmd {
	return textinput.Blink // Reuse Bubbles' blink command for the text input
}

// Update is the main update function that handles all messages and updates the model state.
// It's a core part of the Bubble Tea framework's model interface.
//
// Parameters:
//   - msg: The message to process (key presses, window resizes, etc.)
//
// Returns:
//   - tea.Model: The updated model
//   - tea.Cmd: A command to be executed by the Bubble Tea runtime.
//
// The function handles different types of messages including:
// - Key presses
// - Window resize events
// - Custom messages for streaming responses
// - Error handling
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// log.Println("Received message:", msg)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case types.TokenMsg:
		m.Msg += string(msg)
		// Request an immediate re-render by returning a command that does nothing
		if m.StreamCh != nil && m.ErrCh != nil {
			return m, tea.Batch(
				llm.NextTokenCmd(m.StreamCh, m.ErrCh),
				// Force a re-render
				func() tea.Msg { return nil },
			)
		}

	case types.StreamEndMsg:
		m.handleStreamEnd()

	case types.StreamErrMsg:
		m.handleStreamError()
	}

	return m, nil
}

// handleKeyMsg processes keyboard input messages.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If we're in options mode, handle all keys through handleOptionsKeyPress.
	if m.ScreenMode == types.ModeOptions {
		// If we're editing a field, handle that first.
		switch {
		case m.EditingModel:
			return m.handleModelInput(msg)
		case m.EditingAPIURL:
			return m.handleAPIURLInput(msg)
		default:
			return m.handleOptionsKeyPress(msg)
		}
	}

	// If we're editing any input field, handle that first.
	switch {
	case m.EditingModel:
		return m.handleModelInput(msg)
	case m.EditingAPIURL:
		return m.handleAPIURLInput(msg)
	}

	// Handle chat mode key bindings.
	switch msg.Type {
	case tea.KeyEsc:
		if m.Streaming {
			m.stopStreaming()
			m.Msg = ""
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyCtrlW:
		m.safeCloseChannels()
		return m, tea.Quit

	case tea.KeyEnter:
		if !m.Streaming {
			return m.handleChatInput()
		}

	case tea.KeyRunes, tea.KeyBackspace:
		// Handle regular typing and backspace for the main chat input.
		var cmd tea.Cmd
		m.TextInput, cmd = m.TextInput.Update(msg)
		return m, cmd

	case tea.KeyCtrlO: // Toggle options menu
		m.ScreenMode = types.ModeOptions

	case tea.KeyCtrlA: // Select all text in input
		// Set cursor to end of text
		m.TextInput.SetCursor(len(m.TextInput.Value()))
		// Move cursor to start to select all text
		m.TextInput.CursorStart()

	case tea.KeyCtrlS: // Use Ctrl+S for storing current question
		go m.StoreCurrentQuestion()
	}

	return m, nil
}

// handleOptionsKeyPress handles all key presses when in options mode.
func (m *Model) handleOptionsKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC: // Ctrl+C to go back to chat
		m.ScreenMode = types.ModeChat
		m.SelectedOpt = 0
		return m, nil

	case tea.KeyTab: // Handle Tab and Shift+Tab for navigation
		if m.EditingModel || m.EditingAPIURL {
			break
		}
		// Check for Shift+Tab (some terminals send Alt/other combos; adjust as needed).
		if msg.Alt { // Treat Alt+Tab as "up".
			m.SelectedOpt = (m.SelectedOpt - 1 + len(m.Options)) % len(m.Options)
		} else { // Regular Tab - move down.
			m.SelectedOpt = (m.SelectedOpt + 1) % len(m.Options)
		}
		return m, nil

	case tea.KeyUp, tea.KeyDown: // Handle up/down arrow keys for temperature
		if m.SelectedOpt == 1 && !m.EditingModel && !m.EditingAPIURL {
			if msg.Type == tea.KeyUp {
				m.Temperature = math.Min(m.Temperature+0.1, 2.0)
			} else {
				m.Temperature = math.Max(m.Temperature-0.1, 0.1)
			}
			m.Options[1] = fmt.Sprintf("Temperature: %.1f (use ↑/↓)", m.Temperature)
			return m, nil
		}

	case tea.KeyEnter:
		// Handle option selection with Enter.
		if !m.EditingModel && !m.EditingAPIURL {
			return m.handleOptionsSelection()
		}

	case tea.KeyRunes:
		// Handle other key runes when not editing.
		if !m.EditingModel && !m.EditingAPIURL && len(msg.Runes) == 1 {
			// Reserved for future shortcuts if needed.
			return m, nil
		}
		// Fall through to handle text input if editing.
		fallthrough

	case tea.KeyBackspace:
		// Handle text input for editing model name or API URL.
		switch {
		case m.EditingModel:
			return m.handleModelInput(msg)
		case m.EditingAPIURL:
			return m.handleAPIURLInput(msg)
		}
		return m, nil
	}

	return m, nil
}

// handleModelInput handles input when editing the model name.
func (m *Model) handleModelInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// First, update the model input with the current message.
	var cmd tea.Cmd
	m.ModelInput, cmd = m.ModelInput.Update(msg)

	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC: // Handle both Esc and Ctrl+C to cancel
		m.EditingModel = false
		m.ModelInput.Blur()
		m.ModelInput.Reset()
		m.setStatus("Model change cancelled", 2*time.Second)
		return m, cmd

	case tea.KeyEnter:
		// Save the new model name.
		newModel := strings.TrimSpace(m.ModelInput.Value())
		if newModel != "" {
			// Save to config.
			if err := config.SaveConfig(newModel, m.Temperature, config.APIURL()); err != nil {
				m.setStatus(fmt.Sprintf("Failed to save model: %v", err), 3*time.Second)
			} else {
				m.ModelName = newModel
				m.Options[0] = "Change Model: " + newModel
				m.setStatus(fmt.Sprintf("Model set to %s", newModel), 2*time.Second)
			}
		}
		m.EditingModel = false
		m.ModelInput.Blur()
		m.ModelInput.Reset()
		return m, nil

	default:
		return m, cmd
	}
}

// handleAPIURLInput handles input when editing the API URL.
func (m *Model) handleAPIURLInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.EditingAPIURL = false
		return m, nil

	case tea.KeyEnter:
		newURL := strings.TrimSpace(m.APIURLInput.Value())
		if newURL != "" {
			// Save the configuration with the new API URL.
			if err := config.SaveConfig(m.ModelName, m.Temperature, newURL); err != nil {
				m.setStatus(fmt.Sprintf("Failed to save API URL: %v", err), 3*time.Second)
			} else {
				// Keep "Set API URL" consistently at Options[2].
				m.Options[2] = "Set API URL: " + newURL
				m.setStatus("API URL updated", 2*time.Second)
			}
		}
		m.EditingAPIURL = false
		return m, nil

	default:
		var cmd tea.Cmd
		m.APIURLInput, cmd = m.APIURLInput.Update(msg)
		return m, cmd
	}
}

// handleChatInput handles input when in chat mode.
func (m *Model) handleChatInput() (tea.Model, tea.Cmd) {
	question := m.TextInput.Value()
	if question == "" {
		// log.Println("Empty question, ignoring input")
		return m, nil
	}

	// log.Printf("Processing question: %s", question)
	m.LastQuestion = question
	m.Msg = ""

	// Initialize channels.
	m.ensureChannels()
	m.Streaming = true
	// log.Println("Streaming started")

	// Create a channel to collect the full response.
	responseCh := make(chan string, 1)

	// Start the streaming with the response collector.
	// log.Printf("Starting stream with model: %s, temperature: %.2f", m.ModelName, m.Temperature)
	start := llm.StartStreamCmdWithCallback(
		config.APIURL(), m.ModelName, question, m.Temperature,
		m.StreamCh, m.ErrCh, m.StopCh, responseCh,
	)
	// log.Println("Stream command started")

	// Command to save the full response when it's ready.
	saveCmd := func() tea.Msg {
		// log.Println("Waiting for full response from channel...")
		fullResponse := <-responseCh
		// log.Printf("Received full response (length: %d)", len(fullResponse))

		if fullResponse != "" {
			// Save the conversation to the vault.
			// log.Println("Saving conversation to vault...")
			if err := m.StoreQA(question, fullResponse); err != nil {
				errMsg := fmt.Sprintf("Failed to save conversation: %v", err)
				log.Println(errMsg)
				return types.StatusMsg{
					Message:  "Failed to save conversation",
					Duration: 3 * time.Second,
				}
			}
			// Optional: success status.
			return types.StatusMsg{
				Message:  "Conversation saved to vault",
				Duration: 3 * time.Second,
			}
		}
		return nil
	}

	return m, tea.Batch(start, llm.NextTokenCmd(m.StreamCh, m.ErrCh), saveCmd)
}

// handleOptionsSelection handles option selection in the options menu.
func (m *Model) handleOptionsSelection() (tea.Model, tea.Cmd) {
	switch m.SelectedOpt {
	case 0: // Change Model
		m.EditingModel = true
		m.ModelInput.SetValue(m.ModelName)
		m.ModelInput.Focus()
		return m, textinput.Blink

	case 1: // Toggle Temperature
		// Actual changes handled with Up/Down arrows.
		return m, nil

	case 2: // Set API URL
		m.EditingAPIURL = true
		m.APIURLInput.SetValue(config.APIURL())
		m.APIURLInput.Focus()
		return m, textinput.Blink

	case 3: // Save Settings
		// Get the current API URL from the input field.
		apiURL := m.APIURLInput.Value()
		if apiURL == "" {
			apiURL = config.APIURL()
		}

		// Save the current settings to the config file.
		if err := config.SaveConfig(m.ModelName, m.Temperature, apiURL); err != nil {
			m.setStatus(fmt.Sprintf("Failed to save settings: %v", err), 3*time.Second)
		} else {
			// Update the displayed options with the new values.
			m.Options[0] = "Change Model: " + m.ModelName
			m.Options[2] = "Set API URL: " + apiURL
			m.setStatus("Settings saved successfully!", 2*time.Second)
		}
		return m, nil

	case 4: // Back to Chat
		m.ScreenMode = types.ModeChat
		m.SelectedOpt = 0
		return m, nil

	case 5: // Update Qdrant index
		go m.StoreCurrentQuestion()
		m.ScreenMode = types.ModeChat
		m.SelectedOpt = 0
		return m, nil
	}

	return m, nil
}

// handleStreamEnd handles the end of a stream.
func (m *Model) handleStreamEnd() {
	m.Streaming = false
	m.safeCloseChannels()
	// Force a re-render to update the UI
	m.Update(nil)
}

// handleStreamError handles errors from the stream.
func (m *Model) handleStreamError() {
	m.Streaming = false

	// Get error from error channel if available.
	if m.ErrCh != nil {
		select {
		case err := <-m.ErrCh:
			if err != nil {
				m.Msg = "Error: " + err.Error()
				m.safeCloseChannels()
				// Force a re-render to show the error
				m.Update(nil)
				return
			}
		default:
		}
	}

	m.Msg = "Error: An unknown error occurred during streaming"
	m.safeCloseChannels()
	// Force a re-render to show the error
	m.Update(nil)
}

// ensureChannels initializes the necessary channels if they don't exist.
func (m *Model) ensureChannels() {
	if m.StreamCh == nil {
		m.StreamCh = make(chan string, 64)
	}
	if m.ErrCh == nil {
		m.ErrCh = make(chan error, 1)
	}
	if m.StopCh == nil {
		m.StopCh = make(chan struct{})
	}
}

// stopStreaming safely stops any ongoing streaming.
func (m *Model) stopStreaming() {
	m.Streaming = false
	m.safeCloseChannels()
}

// safeCloseChannels safely closes all channels.
func (m *Model) safeCloseChannels() {
	safeClose := func(ch *chan struct{}) {
		if ch == nil || *ch == nil {
			return
		}
		// Close and nil the channel. We assume this is the only closer.
		close(*ch)
		*ch = nil
	}

	safeCloseErrCh := func(ch *chan error) {
		if ch == nil || *ch == nil {
			return
		}
		// Close and nil the error channel.
		close(*ch)
		*ch = nil
	}

	safeClose(&m.StopCh)
	safeCloseErrCh(&m.ErrCh)

	if m.StreamCh != nil {
		close(m.StreamCh)
		m.StreamCh = nil
	}
}
