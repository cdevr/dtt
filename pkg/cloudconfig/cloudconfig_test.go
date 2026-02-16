package cloudconfig

import (
	"strings"
	"testing"
)

func TestGenerateBasic(t *testing.T) {
	config := &CloudInitConfig{
		Hostname: "test-vm",
		Username: "ubuntu",
	}

	output := config.Generate()

	if !strings.Contains(output, "#cloud-config") {
		t.Error("Expected cloud-config header")
	}

	if !strings.Contains(output, "hostname: test-vm") {
		t.Error("Expected hostname in output")
	}

	if !strings.Contains(output, "name: ubuntu") {
		t.Error("Expected username in output")
	}
}

func TestGenerateWithPackages(t *testing.T) {
	config := &CloudInitConfig{
		Packages: []string{"curl", "wget", "git"},
	}

	output := config.Generate()

	if !strings.Contains(output, "packages:") {
		t.Error("Expected packages section")
	}

	if !strings.Contains(output, "- curl") {
		t.Error("Expected curl package")
	}
}

func TestGenerateWithRunCommands(t *testing.T) {
	config := &CloudInitConfig{
		RunCommands: []string{
			"apt-get update",
			"apt-get upgrade -y",
		},
	}

	output := config.Generate()

	if !strings.Contains(output, "runcmd:") {
		t.Error("Expected runcmd section")
	}

	if !strings.Contains(output, "apt-get update") {
		t.Error("Expected run command in output")
	}
}

func TestBuilder(t *testing.T) {
	config := NewBuilder().
		WithHostname("my-vm").
		WithUsername("cloud-user").
		WithPassword("password123").
		WithPackage("curl").
		WithRunCommand("echo 'Hello World'").
		Build()

	output := config.Generate()

	if !strings.Contains(output, "hostname: my-vm") {
		t.Error("Expected hostname from builder")
	}

	if !strings.Contains(output, "name: cloud-user") {
		t.Error("Expected username from builder")
	}

	if !strings.Contains(output, "- curl") {
		t.Error("Expected package from builder")
	}

	if !strings.Contains(output, "echo 'Hello World'") {
		t.Error("Expected run command from builder")
	}
}

func TestBuilderEmptyConfig(t *testing.T) {
	config := NewBuilder().Build()
	output := config.Generate()

	if !strings.Contains(output, "#cloud-config") {
		t.Error("Expected cloud-config header even for empty config")
	}
}
