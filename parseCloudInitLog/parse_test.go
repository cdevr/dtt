package parseCloudInitLog

import (
	"os"
	"strings"
	"testing"
)

func TestParseCloudInit(t *testing.T) {
	tests := []struct {
		name         string
		filepath     string
		wantHost     string
		wantMinIPs   int
		wantIPs      []string
		wantSshKeys  map[string]SSHKeyData
		wantMinKeys  int
		wantMinHash  int
		skipComplete bool // files that are incomplete (no login prompt)
	}{
		{
			name:       "Debian 11",
			filepath:   "testdata/dtt-debian-11-104-cloudinit.serial.txt",
			wantHost:   "dtt-debian-11-104",
			wantMinIPs: 3,
			wantIPs: []string{
				"192.168.1.191",
				"2a02:aa14:4582:1100:be24:11ff:feb7:e9c1/64",
				"fe80::be24:11ff:feb7:e9c1/64 ",
			},
			wantMinKeys: 3,
			wantMinHash: 4,
		},
		{
			name:       "Ubuntu Bionic",
			filepath:   "testdata/dtt-ubuntu-bionic-105-cloudinit.serial.txt",
			wantHost:   "dtt-ubuntu-bionic-105",
			wantMinIPs: 1,
			wantIPs: []string{
				"192.168.1.26",
				"2a02:aa14:4582:1100:be24:11ff:fe9f:4b0f/64",
				"fe80::be24:11ff:fe9f:4b0f/64",
			},
			wantMinKeys:  0,
			wantMinHash:  0,
			skipComplete: true, // incomplete file
		},
		{
			name:       "Ubuntu Focal",
			filepath:   "testdata/dtt-ubuntu-focal-106-cloudinit.serial.txt",
			wantHost:   "dtt-ubuntu-focal-106",
			wantMinIPs: 2,
			wantIPs: []string{
				"192.168.1.146",
				"fe80::be24:11ff:fe0b:5334/64",
			},
			wantMinKeys:  0,
			wantMinHash:  0,
			skipComplete: true, // incomplete file
		},
		{
			name:       "Ubuntu Jammy",
			filepath:   "testdata/dtt-ubuntu-jammy-107-cloudinit.serial.txt",
			wantHost:   "dtt-ubuntu-jammy-107",
			wantMinIPs: 2,
			wantIPs: []string{
				"192.168.1.148",
				"fe80::be24:11ff:fe8a:ee23/64",
			},
			wantMinKeys: 3,
			wantMinHash: 3,
		},
		{
			name:       "Ubuntu Noble",
			filepath:   "testdata/dtt-ubuntu-noble-108-cloudinit.serial.txt",
			wantHost:   "dtt-ubuntu-noble-108",
			wantMinIPs: 2,
			wantIPs: []string{
				"192.168.1.164",
				"fe80::be24:11ff:fe3c:caa5/64",
			},
			wantMinKeys: 3,
			wantMinHash: 3,
		},
		{
			name:       "Ubuntu Noble with ssh keys",
			filepath:   "testdata/dtt-ubuntu-noble-cloudinit-with-sshkey.serial.txt",
			wantHost:   "dtt-ubuntu-24",
			wantMinIPs: 2,
			wantIPs: []string{
				"192.168.1.42",
				"fe80::be24:11ff:fe47:b4f1/64",
			},
			wantSshKeys: map[string]SSHKeyData{
				"dtt": {
					Keytype:     "ssh-rsa",
					FingerPrint: "0f:f4:bf:31:b8:42:b8:bd:ad:df:cb:c6:02:23:08:c8:93:be:0c:03:61:00:18:9a:6e:7c:7a:d0:2c:b2:5a:27",
					Options:     "",
					Comment:     "cde@shadow",
				},
			},
			wantMinKeys: 3,
			wantMinHash: 3,
		},
		{
			name:       "Debian 13",
			filepath:   "testdata/dtt-debian-13-109-cloudinit.serial.txt",
			wantHost:   "dtt-debian-13-109",
			wantMinIPs: 2,
			wantIPs: []string{
				"192.168.1.169",
				"fe80::be24:11ff:fec1:62c4/64",
			},
			wantMinKeys:  0,
			wantMinHash:  0,
			skipComplete: true, // incomplete file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.filepath)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", tt.filepath, err)
			}

			data := ParseCloudInit(content)

			if !tt.skipComplete && data.Hostname != tt.wantHost {
				t.Errorf("Hostname = %q, want %q", data.Hostname, tt.wantHost)
			}
			if tt.skipComplete && data.Hostname != "" && data.Hostname != tt.wantHost {
				t.Errorf("Hostname = %q, want %q", data.Hostname, tt.wantHost)
			}

			if len(data.IPs) < tt.wantMinIPs {
				t.Errorf("Got %d IPs, want at least %d. IPs: %v", len(data.IPs), tt.wantMinIPs, data.IPs)
			}
			if len(tt.wantIPs) > 0 {
				gotIPs := make(map[string]struct{}, len(data.IPs))
				for _, ip := range data.IPs {
					gotIPs[strings.TrimSpace(ip)] = struct{}{}
				}
				for _, wantIP := range tt.wantIPs {
					if _, ok := gotIPs[strings.TrimSpace(wantIP)]; !ok {
						t.Errorf("Expected IP %q not found in IPs: %v", strings.TrimSpace(wantIP), data.IPs)
					}
				}
			}

			if len(data.HostKeys) < tt.wantMinKeys {
				t.Errorf("Got %d host keys, want at least %d", len(data.HostKeys), tt.wantMinKeys)
			}

			if len(data.HostKeyHashes) < tt.wantMinHash {
				t.Errorf("Got %d host key hashes, want at least %d", len(data.HostKeyHashes), tt.wantMinHash)
			}
			if len(tt.wantSshKeys) > 0 {
				if len(data.SSHKeyData) != len(tt.wantSshKeys) {
					t.Errorf("Got %d SSH key entries, want %d", len(data.SSHKeyData), len(tt.wantSshKeys))
				}
				for user, wantKey := range tt.wantSshKeys {
					gotKey, ok := data.SSHKeyData[user]
					if !ok {
						t.Errorf("Missing SSH key entry for user %q", user)
						continue
					}
					if gotKey != wantKey {
						t.Errorf("SSH key entry for user %q = %+v, want %+v", user, gotKey, wantKey)
					}
				}
			}

			// Verify at least one IPv4 address
			if len(data.IPs) > 0 {
				hasIPv4 := false
				for _, ip := range data.IPs {
					if !strings.Contains(ip, ":") {
						hasIPv4 = true
						break
					}
				}
				if !hasIPv4 {
					t.Error("Expected at least one IPv4 address")
				}
			}

			// Verify host keys are in the expected format
			for _, key := range data.HostKeys {
				if !strings.HasPrefix(key, "ssh-") && !strings.HasPrefix(key, "ecdsa-") {
					t.Errorf("Invalid host key format: %s", key)
				}
			}

			// Verify host key hashes
			for _, hash := range data.HostKeyHashes {
				if hash.Hostname != tt.wantHost {
					t.Errorf("Hash hostname = %q, want %q", hash.Hostname, tt.wantHost)
				}
				if !strings.HasPrefix(hash.Fingerprint, "SHA256:") {
					t.Errorf("Invalid fingerprint format: %s", hash.Fingerprint)
				}
			}
		})
	}
}

func TestParseCloudInitDebian11Detailed(t *testing.T) {
	content, err := os.ReadFile("testdata/dtt-debian-11-104-cloudinit.serial.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	data := ParseCloudInit(content)

	// Check specific values
	if data.Hostname != "dtt-debian-11-104" {
		t.Errorf("Hostname = %q, want %q", data.Hostname, "dtt-debian-11-104")
	}

	// Check that we have the expected IPv4 address
	expectedIP := "192.168.1.191"
	found := false
	for _, ip := range data.IPs {
		if ip == expectedIP {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected IP %s not found in IPs: %v", expectedIP, data.IPs)
	}

	// Check we have RSA, ECDSA, and ED25519 keys
	keyTypes := make(map[string]bool)
	for _, key := range data.HostKeys {
		if strings.HasPrefix(key, "ssh-rsa") {
			keyTypes["rsa"] = true
		} else if strings.HasPrefix(key, "ssh-ed25519") {
			keyTypes["ed25519"] = true
		} else if strings.HasPrefix(key, "ecdsa-") {
			keyTypes["ecdsa"] = true
		}
	}

	expectedTypes := []string{"rsa", "ed25519", "ecdsa"}
	for _, keyType := range expectedTypes {
		if !keyTypes[keyType] {
			t.Errorf("Missing %s key type", keyType)
		}
	}
}
