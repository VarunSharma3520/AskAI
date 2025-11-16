// Package ui provides the terminal user interface components for the AskAI application.
// This file contains style definitions for various UI elements using the lipgloss library.
package ui

import (
	"github.com/VarunSharma3520/AskAI/internal/config"
	"github.com/charmbracelet/lipgloss"
)

// Global style definitions for consistent theming across the application.
// These styles are used to maintain a consistent look and feel.
var (
	// titleStyle defines the styling for the application title/header.
	// It uses the application's main colors with bold text and padding.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(config.MainColorBackground)).
			Background(lipgloss.Color(config.MainColorForeground)).
			PaddingRight(4).
			PaddingLeft(4).
			AlignVertical(lipgloss.Center)

	// helpStyle defines the styling for help/instruction text.
	// It uses a muted version of the background color with italic text.
	helpStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color(config.MainColorBackgroundMute))

	// optionStyle defines the styling for option text in the UI
	optionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			MarginBottom(1)

	// statusStyle defines the styling for status messages
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)
