package cloudconfig

import (
	"fmt"
	"strings"
)

// CloudInitConfig represents cloud-init user-data configuration
type CloudInitConfig struct {
	Hostname    string
	Username    string
	Password    string
	PublicKeys  []string
	Packages    []string
	RunCommands []string
	Environment map[string]string
}

// Generate generates cloud-init user-data YAML
func (c *CloudInitConfig) Generate() string {
	var sb strings.Builder

	sb.WriteString("#cloud-config\n")

	if c.Hostname != "" {
		sb.WriteString(fmt.Sprintf("hostname: %s\n", c.Hostname))
	}

	if c.Username != "" {
		sb.WriteString("users:\n")
		sb.WriteString(fmt.Sprintf("  - name: %s\n", c.Username))

		if c.Password != "" {
			sb.WriteString(fmt.Sprintf("    passwd: %s\n", c.Password))
		}

		if len(c.PublicKeys) > 0 {
			sb.WriteString("    ssh_authorized_keys:\n")
			for _, key := range c.PublicKeys {
				sb.WriteString(fmt.Sprintf("      - %s\n", key))
			}
		}

		sb.WriteString("    sudo: ['ALL=(ALL) NOPASSWD:ALL']\n")
		sb.WriteString("    shell: /bin/bash\n")
	}

	if len(c.Packages) > 0 {
		sb.WriteString("packages:\n")
		for _, pkg := range c.Packages {
			sb.WriteString(fmt.Sprintf("  - %s\n", pkg))
		}
	}

	if len(c.RunCommands) > 0 {
		sb.WriteString("runcmd:\n")
		for _, cmd := range c.RunCommands {
			sb.WriteString(fmt.Sprintf("  - %s\n", cmd))
		}
	}

	if len(c.Environment) > 0 {
		sb.WriteString("write_files:\n")
		for key, value := range c.Environment {
			sb.WriteString(fmt.Sprintf("  - path: /etc/environment.d/%s.conf\n", key))
			sb.WriteString("    content: |\n")
			lines := strings.Split(value, "\n")
			for _, line := range lines {
				sb.WriteString(fmt.Sprintf("      %s\n", line))
			}
		}
	}

	return sb.String()
}

// Builder provides a fluent interface for building cloud-init configurations
type Builder struct {
	config *CloudInitConfig
}

// NewBuilder creates a new cloud-init configuration builder
func NewBuilder() *Builder {
	return &Builder{
		config: &CloudInitConfig{
			PublicKeys:  []string{},
			Packages:    []string{},
			RunCommands: []string{},
			Environment: make(map[string]string),
		},
	}
}

// WithHostname sets the hostname
func (b *Builder) WithHostname(hostname string) *Builder {
	b.config.Hostname = hostname
	return b
}

// WithUsername sets the default user
func (b *Builder) WithUsername(username string) *Builder {
	b.config.Username = username
	return b
}

// WithPassword sets the user password
func (b *Builder) WithPassword(password string) *Builder {
	b.config.Password = password
	return b
}

// WithPublicKey adds a public SSH key
func (b *Builder) WithPublicKey(key string) *Builder {
	b.config.PublicKeys = append(b.config.PublicKeys, key)
	return b
}

// WithPackage adds a package to install
func (b *Builder) WithPackage(pkg string) *Builder {
	b.config.Packages = append(b.config.Packages, pkg)
	return b
}

// WithRunCommand adds a command to run during boot
func (b *Builder) WithRunCommand(cmd string) *Builder {
	b.config.RunCommands = append(b.config.RunCommands, cmd)
	return b
}

// WithEnvironment adds an environment variable configuration
func (b *Builder) WithEnvironment(key, value string) *Builder {
	b.config.Environment[key] = value
	return b
}

// Build returns the configured CloudInitConfig
func (b *Builder) Build() *CloudInitConfig {
	return b.config
}
