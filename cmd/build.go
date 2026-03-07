package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Build rebuilds the bay binary from source and installs to ~/.local/bin/.
func Build() error {
	srcDir := filepath.Join(os.Getenv("HOME"), "workspace", "bay")
	installDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	installPath := filepath.Join(installDir, "bay")

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("creating install dir: %w", err)
	}

	fmt.Println("Building bay...")
	cmd := exec.Command("go", "build", "-o", installPath, ".")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("Installed to " + installPath)
	return nil
}
