package parseCloudInitLog

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// CloudInitData contains the parsed cloud-init information from a VM
type CloudInitData struct {
	Hostname      string
	IPs           []string
	HostKeyHashes []HostKeyHash
	HostKeys      []string
	SSHKeyData    map[string]SSHKeyData
}

// HostKeyHash represents an SSH host key fingerprint
type HostKeyHash struct {
	KeyType     string
	Fingerprint string
	Hostname    string
	Algorithm   string
}

type SSHKeyData struct {
	Keytype     string
	FingerPrint string
	Options     string
	Comment     string
}

var (
	ipv4Regex     = regexp.MustCompile(`\|\s+eth0\s+\|\s+True\s+\|\s+(\d+\.\d+\.\d+\.\d+)\s+\|`)
	ipv6Regex     = regexp.MustCompile(`\|\s+eth0\s+\|\s+True\s+\|\s+([0-9a-f:]+/\d+)\s+\|`)
	hashRegex     = regexp.MustCompile(`(\d+)\s+(SHA256:[A-Za-z0-9+/]+)\s+root@(\S+)\s+\((\w+)\)`)
	hostnameRegex = regexp.MustCompile(`(\S+)\s+login:\s*$`)
	sshKeyRegex   = regexp.MustCompile(`^(ssh-\S+|ecdsa-\S+)\s+\S+\s+root@(\S+)`)
	authKeyUser   = regexp.MustCompile(`^ci-info:\s+\+.*for user ([^+\s]+)\+`)
	authKeyRow    = regexp.MustCompile(`^ci-info:\s+\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|`)
)

// ParseCloudInit parses cloud-init serial output and extracts VM configuration
func ParseCloudInit(content []byte) CloudInitData {
	data := CloudInitData{
		IPs:           []string{},
		HostKeyHashes: []HostKeyHash{},
		HostKeys:      []string{},
		SSHKeyData:    map[string]SSHKeyData{},
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	inHostKeys := false
	currentAuthUser := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Extract hostname from login prompt
		if data.Hostname == "" {
			if matches := hostnameRegex.FindStringSubmatch(line); matches != nil {
				data.Hostname = matches[1]
			}
		}

		// Extract IPv4 addresses
		if matches := ipv4Regex.FindStringSubmatch(line); matches != nil {
			ip := matches[1]
			if !contains(data.IPs, ip) {
				data.IPs = append(data.IPs, ip)
			}
		}

		// Extract IPv6 addresses
		if matches := ipv6Regex.FindStringSubmatch(line); matches != nil {
			ip := matches[1]
			if !contains(data.IPs, ip) {
				data.IPs = append(data.IPs, ip)
			}
		}

		// Extract host key fingerprints
		if matches := hashRegex.FindStringSubmatch(line); matches != nil {
			hash := HostKeyHash{
				KeyType:     matches[4],
				Fingerprint: matches[2],
				Hostname:    matches[3],
				Algorithm:   matches[1] + " bits",
			}
			data.HostKeyHashes = append(data.HostKeyHashes, hash)
		}

		// Extract actual SSH host keys
		if strings.Contains(line, "-----BEGIN SSH HOST KEY KEYS-----") {
			inHostKeys = true
			continue
		}
		if strings.Contains(line, "-----END SSH HOST KEY KEYS-----") {
			inHostKeys = false
			continue
		}
		if inHostKeys {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "ssh-") || strings.HasPrefix(trimmed, "ecdsa-") {
				data.HostKeys = append(data.HostKeys, trimmed)
				// Extract hostname from key if we don't have it yet
				if data.Hostname == "" {
					if matches := sshKeyRegex.FindStringSubmatch(trimmed); matches != nil {
						data.Hostname = matches[2]
					}
				}
			}
		}

		// Extract authorized SSH key metadata for cloud-init users.
		if matches := authKeyUser.FindStringSubmatch(line); matches != nil {
			currentAuthUser = matches[1]
			continue
		}
		if currentAuthUser != "" {
			if strings.HasPrefix(line, "ci-info: +") {
				continue
			}
			if matches := authKeyRow.FindStringSubmatch(line); matches != nil {
				keytype := strings.TrimSpace(matches[1])
				if strings.HasPrefix(keytype, "ssh-") || strings.HasPrefix(keytype, "ecdsa-") {
					options := strings.TrimSpace(matches[3])
					if options == "-" {
						options = ""
					}
					data.SSHKeyData[currentAuthUser] = SSHKeyData{
						Keytype:     keytype,
						FingerPrint: strings.TrimSpace(matches[2]),
						Options:     options,
						Comment:     strings.TrimSpace(matches[4]),
					}
					currentAuthUser = ""
				}
			}
		}
	}

	return data
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
