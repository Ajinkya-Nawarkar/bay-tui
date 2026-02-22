package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Build rebuilds the bay binary from source.
func Build() error {
	srcDir := filepath.Join(os.Getenv("HOME"), "workspace", "bay")

	fmt.Println("Building bay...")
	cmd := exec.Command("go", "build", "-o", filepath.Join(srcDir, "bay"), ".")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("Done. bay is up to date.")
	return nil
}
