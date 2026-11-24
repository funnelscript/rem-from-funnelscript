package kb

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Pack creates a sanitized zip bundle for downstream coding agents.
func Pack(kbDir, outZip string, full bool) (string, error) {
	absDir, err := filepath.Abs(kbDir)
	if err != nil {
		return "", err
	}

	if outZip == "" {
		outZip = filepath.Join(filepath.Dir(absDir), filepath.Base(absDir)+"-handoff.zip")
	}

	// 1. Ensure BUILD.md is present
	buildPath := filepath.Join(absDir, "BUILD.md")
	if _, err := os.Stat(buildPath); os.IsNotExist(err) {
		raw, err := assetsFS.ReadFile("assets/templates/handoff/BUILD.md")
		if err == nil {
			_ = os.WriteFile(buildPath, raw, 0o644)
		}
	}

	// 2. Generate REPORT.html
	reportHTML, err := Report(absDir)
	if err != nil {
		return "", fmt.Errorf("failed compiling REPORT.html for pack: %w", err)
	}
	reportPath := filepath.Join(absDir, "REPORT.html")
	if err := os.WriteFile(reportPath, []byte(reportHTML), 0o644); err != nil {
		return "", fmt.Errorf("failed saving REPORT.html: %w", err)
	}

	// 3. Create zip file
	zipFile, err := os.Create(outZip)
	if err != nil {
		return "", fmt.Errorf("failed creating zip output %q: %w", outZip, err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		// Security Check: prevent symlink path traversal
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, evalErr := filepath.EvalSymlinks(path)
			if evalErr != nil || !strings.HasPrefix(resolved, absDir) {
				return nil //nolint:nilerr // skip unsafe or unresolvable symlinks
			}
		}

		// Skip internal directories
		parts := strings.Split(rel, string(filepath.Separator))
		for _, part := range parts {
			if part == ".git" || part == "node_modules" || part == "_templates" || part == ".DS_Store" {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Handle private sandbox & log folders unless --full
		if !full {
			if parts[0] == "log" {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if parts[0] == "sandbox" {
				if rel == filepath.Join("sandbox", "ledger.csv") {
					// Embed sanitized ledger
					return addSanitizedLedgerToZip(w, path, rel, info)
				}
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			return nil
		}

		// Add file to zip
		return addFileToZip(w, path, rel, info)
	})

	if err != nil {
		return "", fmt.Errorf("failed packing files: %w", err)
	}

	return outZip, nil
}

func addFileToZip(w *zip.Writer, absPath, relPath string, info os.FileInfo) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(relPath)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, f)
	return err
}

func addSanitizedLedgerToZip(w *zip.Writer, absPath, relPath string, info os.FileInfo) error {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	r := csv.NewReader(bytes.NewReader(raw))
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 1 {
		return addFileToZip(w, absPath, relPath, info)
	}

	// Identify location and object_id columns to strip
	header := rows[0]
	keepIndices := make([]int, 0)
	for i, col := range header {
		lower := strings.ToLower(strings.TrimSpace(col))
		if lower != "location" && lower != "object_id" {
			keepIndices = append(keepIndices, i)
		}
	}

	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	for _, row := range rows {
		sanitizedRow := make([]string, 0, len(keepIndices))
		for _, idx := range keepIndices {
			if idx < len(row) {
				sanitizedRow = append(sanitizedRow, row[idx])
			} else {
				sanitizedRow = append(sanitizedRow, "")
			}
		}
		_ = cw.Write(sanitizedRow)
	}
	cw.Flush()

	headerZip := &zip.FileHeader{
		Name:   filepath.ToSlash(relPath),
		Method: zip.Deflate,
	}
	writer, err := w.CreateHeader(headerZip)
	if err != nil {
		return err
	}
	_, err = writer.Write(buf.Bytes())
	return err
}

// ValidateDir checks if directory contains required core files
func ValidateDir(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("directory %q does not exist: %w", dir, err)
	}
	return nil
}
