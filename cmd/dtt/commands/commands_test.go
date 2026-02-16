package commands

import (
	"testing"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()

	if cmd == nil {
		t.Fatal("Expected root command to be created")
	}

	if cmd.Use != "dtt" {
		t.Errorf("Expected use to be 'dtt', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected short description")
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	cmd := NewRootCommand()

	subcommands := []string{"run", "image", "vm", "completion"}
	foundCount := 0

	for _, sub := range cmd.Commands() {
		for _, expected := range subcommands {
			if sub.Name() == expected {
				foundCount++
			}
		}
	}

	if foundCount != len(subcommands) {
		t.Errorf("Expected %d subcommands, found %d", len(subcommands), foundCount)
	}
}

func TestRootCommandGlobalFlags(t *testing.T) {
	cmd := NewRootCommand()

	flags := []string{"proxmox-host", "proxmox-port", "proxmox-user", "proxmox-node"}

	for _, flagName := range flags {
		flag := cmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag %s to exist", flagName)
		}
	}
}

func TestImageListCommand(t *testing.T) {
	cmd := NewImageListCommand()

	if cmd == nil {
		t.Fatal("Expected image list command to be created")
	}

	if cmd.Name() != "list" {
		t.Errorf("Expected command name 'list', got '%s'", cmd.Name())
	}
}

func TestVMListCommand(t *testing.T) {
	cmd := NewVMListCommand()

	if cmd == nil {
		t.Fatal("Expected vm list command to be created")
	}

	if cmd.Name() != "list" {
		t.Errorf("Expected command name 'list', got '%s'", cmd.Name())
	}
}

func TestRunCommandFlags(t *testing.T) {
	cmd := NewRunCommand()

	flags := []string{"hostname", "image", "memory", "cpu", "cores", "username", "remote-path"}

	for _, flagName := range flags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag %s to exist", flagName)
		}
	}
}
