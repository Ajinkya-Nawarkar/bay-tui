package cmd

import (
	"bufio"
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

const releasesAPI = "https://api.github.com/repos/Ajinkya-Nawarkar/bay-tui/releases"

type ghReleaseFull struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	HTMLURL    string    `json:"html_url"`
	Assets     []ghAsset `json:"assets"`
}

// Beta installs the latest pre-release build.
func Beta() error {
	fmt.Println("Checking for beta releases...")

	resp, err := http.Get(releasesAPI)
	if err != nil {
		return fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []ghReleaseFull
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return fmt.Errorf("parsing releases: %w", err)
	}

	// Find the newest pre-release
	var beta *ghReleaseFull
	for i := range releases {
		if releases[i].Prerelease {
			beta = &releases[i]
			break
		}
	}

	if beta == nil {
		fmt.Println("No beta releases available.")
		return nil
	}

	// Find the right asset for this OS/arch
	target := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	var assetURL string
	for _, a := range beta.Assets {
		if strings.Contains(a.Name, target) && strings.HasSuffix(a.Name, ".tar.gz") {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("no beta asset found for %s", target)
	}

	fmt.Printf("\nBeta version: %s\n", beta.TagName)
	fmt.Printf("Release:      %s\n", beta.HTMLURL)
	fmt.Print("\nInstall this beta? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	installDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("creating install dir: %w", err)
	}
	currentBin := filepath.Join(installDir, "bay")

	fmt.Printf("Downloading %s...\n", beta.TagName)

	tmpDir, err := os.MkdirTemp("", "bay-beta-*")
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

	extract := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir)
	if err := extract.Run(); err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	newBin := filepath.Join(tmpDir, "bay")

	if err := os.Rename(newBin, currentBin); err != nil {
		// Cross-device rename; fall back to copy
		src, err := os.Open(newBin)
		if err != nil {
			return fmt.Errorf("opening new binary: %w", err)
		}
		defer src.Close()

		dst, err := os.OpenFile(currentBin, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o755)
		if err != nil {
			return fmt.Errorf("opening current binary for write: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("replacing binary: %w", err)
		}
	}

	fmt.Printf("Installed beta %s\n", beta.TagName)
	fmt.Println("Run 'bay upgrade' to return to the latest stable release.")
	return nil
}
