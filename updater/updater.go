package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// Update checks for the latest release and replaces the current binary if a newer version is available.
func Update(currentVersion string) error {
	fmt.Println("Checking for updates...")

	latest, err := getLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestClean := strings.TrimPrefix(latest, "v")
	if currentVersion == latestClean {
		fmt.Printf("Already up to date (v%s).\n", currentVersion)
		return nil
	}

	fmt.Printf("Updating from v%s to v%s...\n", currentVersion, latestClean)

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	archiveName := fmt.Sprintf("dbtree_%s_%s_%s.tar.gz", latestClean, goos, goarch)
	downloadURL := fmt.Sprintf("https://github.com/viveknathani/dbtree/releases/download/%s/%s", latest, archiveName)

	tmpDir, err := os.MkdirTemp("", "dbtree-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(archivePath, downloadURL); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	binaryPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to extract update: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if err := replaceBinary(execPath, binaryPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Successfully updated to v%s!\n", latestClean)
	return nil
}

func getLatestVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/viveknathani/dbtree/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no release found")
	}

	return release.TagName, nil
}

func downloadFile(dest, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "dbtree" {
			outPath := filepath.Join(destDir, "dbtree")
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
			return outPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

func replaceBinary(target, source string) error {
	// Rename-based atomic replacement
	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	if err := os.Rename(source, target); err != nil {
		// Cross-device fallback: copy then replace
		src, err := os.Open(source)
		if err != nil {
			return err
		}
		defer src.Close()

		tmp := target + ".new"
		dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			os.Remove(tmp)
			return err
		}
		dst.Close()

		if err := os.Rename(tmp, target); err != nil {
			os.Remove(tmp)
			return err
		}
	}

	return os.Chmod(target, info.Mode())
}
