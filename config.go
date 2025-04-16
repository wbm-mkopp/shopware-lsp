package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func getProjectConfigFolder(projectRoot string) (string, error) {
	configDir, err := getUserConfigDir()
	if err != nil {
		return "", err
	}

	projectSlug := strings.ReplaceAll(projectRoot, "/", "_")
	projectSlug = strings.ReplaceAll(projectSlug, ":", "_")
	projectSlug = strings.ReplaceAll(projectSlug, "\\", "_")

	expectedDir := filepath.Join(configDir, "shopware-lsp", projectSlug)

	if _, err := os.Stat(expectedDir); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check directory: %w", err)
		}
		// Directory does not exist, create it
		err = os.MkdirAll(expectedDir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return expectedDir, nil
}

func getUserConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("failed to get current user: %w", err)
		}
		return filepath.Join(usr.HomeDir, ".config"), nil
	}
	return configDir, nil
}
