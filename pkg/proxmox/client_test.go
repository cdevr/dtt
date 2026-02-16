package proxmox

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	config := ClientConfig{
		Host:     "localhost",
		Port:     8006,
		Username: "root@pam",
		Password: "password",
		Node:     "pve",
	}

	client := NewClient(config)

	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.config.Host != config.Host {
		t.Errorf("Expected host %s, got %s", config.Host, client.config.Host)
	}
}

func TestDefaultImages(t *testing.T) {
	images := DefaultImages()

	if len(images) == 0 {
		t.Fatal("Expected default images to be returned")
	}

	expectedNames := []string{"Debian 11", "Debian 13", "Ubuntu 24.04 LTS"}
	for i, expected := range expectedNames {
		if i >= len(images) {
			t.Errorf("Expected at least %d images, got %d", len(expectedNames), len(images))
			break
		}
		if images[i].Name != expected {
			t.Errorf("Expected image name %s, got %s", expected, images[i].Name)
		}
	}
}

func TestVMSpecValidation(t *testing.T) {
	client := NewClient(ClientConfig{
		Host: "localhost",
		Node: "pve",
	})

	spec := VMSpec{
		Name:   "test-vm",
		VMID:   0, // Invalid VMID
		Memory: 512,
		CPU:    1,
	}

	_, err := client.CreateVM(spec)
	if err == nil {
		t.Error("Expected error for invalid VMID")
	}
}

func TestGetVMValidation(t *testing.T) {
	client := NewClient(ClientConfig{
		Host: "localhost",
		Node: "pve",
	})

	_, err := client.GetVM(0) // Invalid VMID
	if err == nil {
		t.Error("Expected error for invalid VMID")
	}
}

func TestDownloadImageValidation(t *testing.T) {
	client := NewClient(ClientConfig{
		Host: "localhost",
		Node: "pve",
	})

	image := Image{
		Name: "Test Image",
		URL:  "",
	}

	err := client.DownloadImage(image, "local")
	if err == nil {
		t.Error("Expected error for missing image URL")
	}

	image.URL = "https://example.com/image.iso"
	err = client.DownloadImage(image, "")
	if err == nil {
		t.Error("Expected error for missing storage ID")
	}
}
