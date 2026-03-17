package updater

import (
	"archive/tar"
	"archive/zip"
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

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	archiveName := fmt.Sprintf("dbtree_%s_%s_%s.%s", latestClean, goos, goarch, ext)
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

	var binaryPath string
	if goos == "windows" {
		binaryPath, err = extractBinaryFromZip(archivePath, tmpDir)
	} else {
		binaryPath, err = extractBinaryFromTarGz(archivePath, tmpDir)
	}
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

func extractBinaryFromTarGz(archivePath, destDir string) (string, error) {
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

		name := filepath.Base(header.Name)
		if header.Typeflag == tar.TypeReg && strings.TrimSuffix(name, ".exe") == "dbtree" {
			outPath := filepath.Join(destDir, name)
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

func extractBinaryFromZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if strings.TrimSuffix(name, ".exe") == "dbtree" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			outPath := filepath.Join(destDir, name)
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				rc.Close()
				return "", err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				rc.Close()
				return "", err
			}
			out.Close()
			rc.Close()
			return outPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

func replaceBinary(target, source string) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	// On Windows, a running executable is locked against overwrite but can
	// still be renamed. Move the running binary aside first, then place the
	// new one at the original path. The .old file is left behind and cleaned
	// up on the next update.
	if runtime.GOOS == "windows" {
		oldPath := target + ".old"
		os.Remove(oldPath) // clean up from a previous update
		if err := os.Rename(target, oldPath); err != nil {
			return fmt.Errorf("failed to move current binary aside: %w", err)
		}
		if err := moveFile(source, target, info.Mode()); err != nil {
			// Try to restore the original binary on failure.
			os.Rename(oldPath, target)
			return err
		}
		return nil
	}

	// Unix: atomic rename into place.
	if err := os.Rename(source, target); err != nil {
		// Cross-device fallback: copy then rename.
		if err := moveFile(source, target, info.Mode()); err != nil {
			return err
		}
	}

	return os.Chmod(target, info.Mode())
}

// moveFile copies source to a temp file next to target, then renames it into place.
func moveFile(source, target string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	tmp := target + ".new"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
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
	return nil
}
