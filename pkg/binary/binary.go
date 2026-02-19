package binary

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BinaryInfo contains metadata about a binary
type BinaryInfo struct {
	Path      string
	Name      string
	Size      int64
	Mode      os.FileMode
	MD5Hash   string
	SHA256Hash string
}

// GetBinaryInfo retrieves information about a binary file
func GetBinaryInfo(path string) (*BinaryInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat binary: %w", err)
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("binary is not a regular file")
	}

	// Calculate hashes
	md5Hash, sha256Hash, err := calculateHashes(path)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hashes: %w", err)
	}

	return &BinaryInfo{
		Path:       path,
		Name:       filepath.Base(path),
		Size:       info.Size(),
		Mode:       info.Mode(),
		MD5Hash:    md5Hash,
		SHA256Hash: sha256Hash,
	}, nil
}

// ValidateBinary checks if a binary is valid and executable
func ValidateBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file")
	}

	// Check if it's executable or has execute bits
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	return nil
}

// VerifyBinary verifies a binary against expected hash values
func VerifyBinary(path string, expectedMD5, expectedSHA256 string) error {
	info, err := GetBinaryInfo(path)
	if err != nil {
		return err
	}

	if expectedMD5 != "" && info.MD5Hash != expectedMD5 {
		return fmt.Errorf("MD5 hash mismatch: expected %s, got %s", expectedMD5, info.MD5Hash)
	}

	if expectedSHA256 != "" && info.SHA256Hash != expectedSHA256 {
		return fmt.Errorf("SHA256 hash mismatch: expected %s, got %s", expectedSHA256, info.SHA256Hash)
	}

	return nil
}

// calculateHashes calculates MD5 and SHA256 hashes for a file
func calculateHashes(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	md5Hash := md5.New()
	sha256Hash := sha256.New()
	multiWriter := io.MultiWriter(md5Hash, sha256Hash)

	if _, err := io.Copy(multiWriter, file); err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%x", md5Hash.Sum(nil)), fmt.Sprintf("%x", sha256Hash.Sum(nil)), nil
}

// RemoteLocation represents a location on the remote VM
type RemoteLocation struct {
	Path        string
	Owner       string
	Group       string
	Permissions int
}

// TransferConfig contains configuration for binary transfer
type TransferConfig struct {
	LocalPath     string
	RemotePath    string
	Owner         string
	Group         string
	Permissions   int
	Timeout       int // in seconds
	Retry         int
	VerifyAfter   bool
}

// ValidateTransferConfig validates transfer configuration
func ValidateTransferConfig(config TransferConfig) error {
	if config.LocalPath == "" {
		return fmt.Errorf("local path is required")
	}

	if config.RemotePath == "" {
		return fmt.Errorf("remote path is required")
	}

	if err := ValidateBinary(config.LocalPath); err != nil {
		return fmt.Errorf("invalid local binary: %w", err)
	}

	if config.Owner == "" {
		config.Owner = "root"
	}

	if config.Group == "" {
		config.Group = "root"
	}

	if config.Permissions == 0 {
		config.Permissions = 0755
	}

	return nil
}