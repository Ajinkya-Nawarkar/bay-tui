package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const repoAPI = "https://api.github.com/repos/Ajinkya-Nawarkar/bay-tui/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Upgrade downloads the latest release binary and replaces the current one.
func Upgrade() error {
	// Find where the current binary lives
	currentBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return fmt.Errorf("resolving symlink: %w", err)
	}

	fmt.Println("Fetching latest release...")
	resp, err := http.Get(repoAPI)
	if err != nil {
		return fmt.Errorf("fetching release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parsing release: %w", err)
	}

	// Find the right asset for this OS/arch
	target := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	var assetURL string
	for _, a := range release.Assets {
		if strings.Contains(a.Name, target) && strings.HasSuffix(a.Name, ".tar.gz") {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("no release asset found for %s", target)
	}

	fmt.Printf("Downloading %s...\n", release.TagName)

	// Download to temp file
	tmpDir, err := os.MkdirTemp("", "bay-upgrade-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "bay.tar.gz")
	dlResp, err := http.Get(assetURL)
	if err != nil {
		return fmt.Errorf("downloading asset: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", dlResp.StatusCode)
	}

	out, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := io.Copy(out, dlResp.Body); err != nil {
		out.Close()
		return fmt.Errorf("writing download: %w", err)
	}
	out.Close()

	// Extract
	extract := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir)
	if err := extract.Run(); err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	newBin := filepath.Join(tmpDir, "bay")

	// Replace current binary
	if err := os.Rename(newBin, currentBin); err != nil {
		// Cross-device rename; fall back to copy
		src, err := os.Open(newBin)
		if err != nil {
			return fmt.Errorf("opening new binary: %w", err)
		}
		defer src.Close()

		dst, err := os.OpenFile(currentBin, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
		if err != nil {
			return fmt.Errorf("opening current binary for write: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("replacing binary: %w", err)
		}
	}

	fmt.Printf("Upgraded to %s\n", release.TagName)
	return nil
}
