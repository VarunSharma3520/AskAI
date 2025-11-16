// Package fs provides file system utilities for the AskAI application.
// It handles operations related to the application's vault directory and file management.
package fs

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureVaultExists checks if the specified vault directory exists and is accessible.
// If the directory doesn't exist, it creates it with default permissions (0755).
//
// Parameters:
//   - path: The filesystem path where the vault should be located
//
// Returns:
//   - error: An error if the vault cannot be created or accessed, or if the path exists but is not a directory
//
// Example:
//   err := fs.EnsureVaultExists("/path/to/vault")
//   if err != nil {
//       log.Fatalf("Failed to initialize vault: %v", err)
//   }
func EnsureVaultExists(path string) error {
	// Check if the path exists
	info, err := os.Stat(path)
	
	// If the path doesn't exist, create it
	if os.IsNotExist(err) {
		// Create all necessary directories in the path with read/write/execute permissions for owner,
		// and read/execute permissions for group and others
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create vault directory: %w", err)
		}
		return nil
	}
	
	// If there was an error checking the path that isn't "not exists"
	if err != nil {
		return fmt.Errorf("failed to check vault directory: %w", err)
	}
	
	// If the path exists but isn't a directory
	if !info.IsDir() {
		return fmt.Errorf("vault path exists but is not a directory: %s", path)
	}
	
	// Check if we have write permissions
	if info.Mode().Perm()&0200 == 0 {
		return fmt.Errorf("insufficient permissions to write to vault directory: %s", path)
	}
	
	return nil
}

// FileExists checks if a file exists and is not a directory.
//
// Parameters:
//   - path: The path to the file to check
//
// Returns:
//   - bool: true if the file exists and is not a directory, false otherwise
//   - error: Any error that occurred during the check
func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking file: %w", err)
	}
	return !info.IsDir(), nil
}

// EnsureFileExists ensures that a file exists at the given path.
// If the file doesn't exist, it creates an empty file.
//
// Parameters:
//   - path: The path where the file should exist
//
// Returns:
//   - error: An error if the file cannot be created or accessed
func EnsureFileExists(path string) error {
	exists, err := fileExists(path)
	if err != nil {
		return fmt.Errorf("error checking file existence: %w", err)
	}
	
	if exists {
		return nil
	}
	
	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}
	
	// Create an empty file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	file.Close()
	
	return nil
}
