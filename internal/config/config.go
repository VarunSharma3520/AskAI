// Package config provides configuration management for the AskAI application.
// It handles application settings, environment variables, and default values.
// Configuration can be customized through environment variables or uses sensible defaults.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ScreenMode represents the different UI modes of the application
type ScreenMode string

// UI color constants for the TUI (Terminal User Interface)
const (
	// MainColorForeground is the primary text color (ANSI color code)
	MainColorForeground = "205"
	// MainColorBackground is the primary background color (ANSI color code)
	MainColorBackground = "16"
	// MainColorBackgroundMute is a muted background color (ANSI color code)
	MainColorBackgroundMute = "241"
)

// Application screen modes
const (
	// ModeChat represents the main chat interface
	ModeChat ScreenMode = "chat"
	// ModeSetting represents the settings interface
	ModeSetting ScreenMode = "setting"
)

// Default configuration values
const (
	// Default directory name for storing application data
	defaultVaultDir = ".askAI"
	// Default URL for the Ollama API server
	defaultAPIURL = "http://localhost:11434"
	// Default model to use for AI completions
	defaultModel = "gemma3:1b"
	// Default temperature for AI responses (higher = more creative, lower = more focused)
	defaultTemp = 1.5
)

// getDefaultVaultPath returns the default path for the vault directory.
// It uses the user's home directory if available, otherwise falls back to the current directory.
func getDefaultVaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return "./" + defaultVaultDir
	}
	return filepath.Join(home, defaultVaultDir)
}

// VaultPath returns the path to the application's data directory.
// It checks the ASKAI_VAULT environment variable first, then falls back to the default.
// The directory will be created if it doesn't exist.
//
// Returns:
//   - string: Path to the vault directory
func VaultPath() string {
	if v := os.Getenv("ASKAI_VAULT"); v != "" {
		return v
	}
	return getDefaultVaultPath()
}

// APIURL returns the base URL for the Ollama API.
// It checks the OLLAMA_API_URL environment variable first, then falls back to the default.
//
// Returns:
//   - string: Base URL of the Ollama API
func APIURL() string {
	if v := os.Getenv("OLLAMA_API_URL"); v != "" {
		return v
	}
	return defaultAPIURL
}

// Model returns the name of the AI model to use.
// It checks the OLLAMA_MODEL environment variable first, then falls back to the default.
//
// Returns:
//   - string: Name of the AI model
func Model() string {
	if v := os.Getenv("OLLAMA_MODEL"); v != "" {
		return v
	}
	return defaultModel
}

// Temperature returns the default temperature setting for AI responses.
// Higher values (closer to 2.0) make output more random, while lower values (closer to 0.0)
// make it more focused and deterministic.
//
// Returns:
//   - float64: Temperature value between 0.0 and 2.0
func Temperature() float64 {
	return defaultTemp
}

// Config represents the application's configuration that can be saved and loaded
type Config struct {
	ModelName   string  `json:"model_name"`
	Temperature float64 `json:"temperature"`
	APIURL      string  `json:"api_url,omitempty"`
}

// SaveConfig saves the current configuration to a file in the vault directory
func SaveConfig(modelName string, temperature float64, apiURL string) error {
	config := Config{
		ModelName:   modelName,
		Temperature: temperature,
		APIURL:      apiURL,
	}

	// Create the config file path
	configPath := filepath.Join(VaultPath(), "config.json")

	// Create the vault directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Marshal the config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write the config file
	return os.WriteFile(configPath, data, 0600)
}

// LoadConfig loads the configuration from the config file if it exists
func LoadConfig() (string, float64, string, error) {
	configPath := filepath.Join(VaultPath(), "config.json")

	// If config file doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return defaultModel, defaultTemp, defaultAPIURL, nil
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", 0, "", err
	}

	// Unmarshal the config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return "", 0, "", err
	}

	// Handle missing API URL in config file
	if config.APIURL == "" {
		config.APIURL = defaultAPIURL
	}

	return config.ModelName, config.Temperature, config.APIURL, nil
}
