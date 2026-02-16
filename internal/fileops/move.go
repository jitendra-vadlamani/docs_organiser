package fileops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveFile moves a file from src to dst.
// It handles cross-device moves by falling back to Copy+Delete.
// It handles collisions by appending a content hash to the filename.
func MoveFile(src, dstFolder string, newFilename string) error {
	dstPath := filepath.Join(dstFolder, newFilename)

	// Ensure destination directory exists (including any subdirectories in newFilename)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Check for collision
	if _, err := os.Stat(dstPath); err == nil {
		// File exists, append hash
		hash, err := getFileHash(src)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for collision resolution: %w", err)
		}
		ext := filepath.Ext(newFilename)
		name := newFilename[:len(newFilename)-len(ext)]
		dstPath = filepath.Join(dstFolder, fmt.Sprintf("%s_%s%s", name, hash[:8], ext))
	}

	// Try atomic rename first
	err := os.Rename(src, dstPath)
	if err == nil {
		return nil
	}

	// If rename fails (likely cross-device), try Copy + Remove
	// Check if it's a cross-device error or something else that permits retry
	// os.Rename returns slightly different errors depending on OS, but generally we just try fallback.

	if err := copyFile(src, dstPath); err != nil {
		return fmt.Errorf("failed to copy file (fallback): %w", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file after copy: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func getFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
