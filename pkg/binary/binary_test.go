package binary

import (
	"os"
	"testing"
	"io"
)

func TestGetBinaryInfo(t *testing.T) {
	// Create a temporary test binary
	tmpFile, err := os.CreateTemp("", "test-binary")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Make it executable
	os.Chmod(tmpFile.Name(), 0755)
	tmpFile.WriteString("#!/bin/bash\necho 'test'\n")
	tmpFile.Close()

	info, err := GetBinaryInfo(tmpFile.Name())
	if err != nil {
		t.Fatalf("GetBinaryInfo failed: %v", err)
	}

	if info.Name == "" {
		t.Error("Expected binary name")
	}

	if info.Size == 0 {
		t.Error("Expected binary size to be greater than 0")
	}

	if info.MD5Hash == "" {
		t.Error("Expected MD5 hash")
	}

	if info.SHA256Hash == "" {
		t.Error("Expected SHA256 hash")
	}
}

func TestValidateBinary(t *testing.T) {
	// Test with non-existent file
	err := ValidateBinary("/nonexistent/binary")
	if err == nil {
		t.Error("Expected error for non-existent binary")
	}

	// Create a valid executable
	tmpFile, err := os.CreateTemp("", "valid-binary")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("#!/bin/bash\n")
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0755)

	err = ValidateBinary(tmpFile.Name())
	if err != nil {
		t.Errorf("ValidateBinary failed for valid binary: %v", err)
	}

	// Test with non-executable file
	nonExecFile, err := os.CreateTemp("", "non-exec")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(nonExecFile.Name())

	nonExecFile.WriteString("test")
	nonExecFile.Close()
	os.Chmod(nonExecFile.Name(), 0644)

	err = ValidateBinary(nonExecFile.Name())
	if err == nil {
		t.Error("Expected error for non-executable file")
	}
}

func TestVerifyBinary(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "verify-binary")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("test content")
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0755)

	info, err := GetBinaryInfo(tmpFile.Name())
	if err != nil {
		t.Fatalf("GetBinaryInfo failed: %v", err)
	}

	// Verify with correct hash
	err = VerifyBinary(tmpFile.Name(), info.MD5Hash, info.SHA256Hash)
	if err != nil {
		t.Errorf("VerifyBinary failed with correct hash: %v", err)
	}

	// Verify with wrong MD5
	err = VerifyBinary(tmpFile.Name(), "wronghash", "")
	if err == nil {
		t.Error("Expected error for wrong MD5 hash")
	}

	// Verify with wrong SHA256
	err = VerifyBinary(tmpFile.Name(), "", "wronghash")
	if err == nil {
		t.Error("Expected error for wrong SHA256 hash")
	}
}

func TestValidateTransferConfig(t *testing.T) {
	// Create a valid binary
	tmpFile, err := os.CreateTemp("", "transfer-binary")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("test")
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0755)

	// Test with missing local path
	config := TransferConfig{
		LocalPath:  "",
		RemotePath: "/tmp/binary",
	}
	err = ValidateTransferConfig(config)
	if err == nil {
		t.Error("Expected error for missing local path")
	}

	// Test with missing remote path
	config = TransferConfig{
		LocalPath:  tmpFile.Name(),
		RemotePath: "",
	}
	err = ValidateTransferConfig(config)
	if err == nil {
		t.Error("Expected error for missing remote path")
	}

	// Test with valid config
	config = TransferConfig{
		LocalPath:  tmpFile.Name(),
		RemotePath: "/tmp/binary",
	}
	err = ValidateTransferConfig(config)
	if err != nil {
		t.Errorf("ValidateTransferConfig failed: %v", err)
	}
}