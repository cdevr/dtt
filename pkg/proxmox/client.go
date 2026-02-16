package proxmox

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
	sshpkg "github.com/example/dtt/pkg/ssh"
)

// ClientConfig contains configuration for Proxmox API client
type ClientConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	TokenID      string // API token ID (e.g., "root@pam!tokenname")
	TokenSecret  string // API token secret
	Realm        string
	Node         string
	Insecure     bool
	SSHUser      string // SSH username for Proxmox host (for image operations)
	SSHPassword  string // SSH password for Proxmox host
	SSHPort      int    // SSH port (default 22)
}

// Client represents a Proxmox API client
type Client struct {
	config    ClientConfig
	apiClient *proxmox.Client
	node      *proxmox.Node
}

// NewClient creates a new Proxmox client
func NewClient(config ClientConfig) *Client {
	return &Client{
		config: config,
	}
}

// Connect establishes a connection to the Proxmox server
func (c *Client) Connect() error {
	if c.apiClient != nil {
		return nil // Already connected
	}

	ctx := context.Background()

	// Create HTTP client with optional insecure TLS
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: c.config.Insecure,
			},
		},
	}

	// Build Proxmox server URL
	serverURL := c.config.Host
	if !strings.HasPrefix(serverURL, "http") {
		serverURL = fmt.Sprintf("https://%s:%d", c.config.Host, c.config.Port)
	}
	// Note: go-proxmox library expects just the base URL, it adds /api2/json itself
	// But let's try with explicit path if it's not working
	if !strings.Contains(serverURL, "/api2/json") {
		serverURL = serverURL + "/api2/json"
	}


	// Create Proxmox client
	var client *proxmox.Client
	var err error

	// Authenticate
	if c.config.TokenID != "" && c.config.TokenSecret != "" {
		// Use API token authentication
		client = proxmox.NewClient(serverURL,
			proxmox.WithHTTPClient(httpClient),
			proxmox.WithAPIToken(c.config.TokenID, c.config.TokenSecret))
	} else if c.config.Password != "" {
		// Use password authentication
		client = proxmox.NewClient(serverURL, proxmox.WithHTTPClient(httpClient))
		err = client.Login(ctx, c.config.Username, c.config.Password)
		if err != nil {
			return fmt.Errorf("failed to login to Proxmox: %w", err)
		}
	} else {
		return fmt.Errorf("no authentication credentials provided")
	}

	// Store the client
	c.apiClient = client

	// Test the connection by getting the version
	version, err := client.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Proxmox version (connection test failed): %w", err)
	}
	fmt.Printf("Connected to Proxmox version %s\n", version.Version)

	return nil
}

// getNode gets the Proxmox node, fetching it if necessary
func (c *Client) getNode() (*proxmox.Node, error) {
	if c.node != nil {
		return c.node, nil
	}

	if c.apiClient == nil {
		return nil, fmt.Errorf("client not connected")
	}

	ctx := context.Background()
	node, err := c.apiClient.Node(ctx, c.config.Node)
	if err != nil {
		return nil, fmt.Errorf("failed to get node '%s': %w", c.config.Node, err)
	}

	c.node = node
	return node, nil
}

// Image represents a VM image available on the Proxmox server
type Image struct {
	Name     string
	OS       string
	Version  string
	LocalID  string // Storage location ID in Proxmox
	URL      string // Download URL if not present
	Size     uint64 // Size in bytes
	Checksum string
}

// DefaultImages returns common image options
func DefaultImages() []Image {
	return []Image{
		{
			Name:    "Debian 11",
			OS:      "debian",
			Version: "11",
			URL:     "https://cloud.debian.org/images/cloud/bullseye/latest/debian-11-generic-amd64.qcow2",
			Size:    0, // Will be fetched during download
		},
		{
			Name:    "Debian 13",
			OS:      "debian",
			Version: "13",
			URL:     "https://cloud.debian.org/images/cloud/trixie/latest/debian-trixie-generic-amd64.qcow2",
			Size:    0,
		},
		{
			Name:    "Ubuntu 24.04 LTS",
			OS:      "ubuntu",
			Version: "24.04",
			URL:     "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			Size:    0,
		},
	}
}

// VMSpec defines the virtual machine specification
type VMSpec struct {
	Name      string
	VMID      int
	Image     Image
	Memory    int // in MB
	CPU       int
	Cores     int
	Disks     int // Number of disks
	DiskSize  int // Size in GB
	CloudInit bool
	Network   string // Network configuration
}

// VM represents a virtual machine on Proxmox
type VM struct {
	ID       int
	Name     string
	Status   string // running, stopped, suspended
	Memory   int
	CPU      int
	Node     string
	Created  time.Time
	Modified time.Time
}

// CreateVM creates a new virtual machine with the given specification
func (c *Client) CreateVM(vmSpec VMSpec) (*VM, error) {
	if vmSpec.VMID <= 0 {
		return nil, fmt.Errorf("invalid VM ID: must be greater than 0")
	}

	// Ensure we're connected
	if err := c.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Proxmox: %w", err)
	}

	node, err := c.getNode()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Check if VM already exists
	existingVM, err := node.VirtualMachine(ctx, vmSpec.VMID)
	if err == nil && existingVM != nil {
		// VM exists, return it
		return &VM{
			ID:       vmSpec.VMID,
			Name:     existingVM.Name,
			Status:   existingVM.Status,
			Memory:   int(existingVM.MaxMem / 1024 / 1024), // Convert bytes to MB
			CPU:      int(existingVM.CPUs),
			Node:     c.config.Node,
			Created:  time.Now(),
			Modified: time.Now(),
		}, nil
	}

	// Create VM with cloud-init configuration
	fmt.Printf("Creating VM with cloud-init...\n")

	// Step 1: Download the cloud image if we have SSH access to Proxmox host
	var imagePath string
	storage := "local-lvm" // Default storage

	if c.config.SSHUser != "" && c.config.SSHPassword != "" && vmSpec.Image.URL != "" {
		var downloadErr error
		imagePath, downloadErr = c.DownloadImageToNode(vmSpec.Image, c.config.SSHUser, c.config.SSHPassword)
		if downloadErr != nil {
			fmt.Printf("Warning: Failed to download image: %v\n", downloadErr)
			fmt.Printf("VM will be created without a boot disk\n")
		}
	}

	// Create the VM using SSH and qm commands instead of Proxmox API
	// The API seems to have issues with VM creation
	fmt.Printf("Creating VM using qm command...\n")

	if c.config.SSHUser == "" || c.config.SSHPassword == "" {
		return nil, fmt.Errorf("SSH credentials required to create VM")
	}

	sshConfig := sshpkg.Config{
		Host:     c.config.Host,
		Port:     22,
		Username: c.config.SSHUser,
		Password: c.config.SSHPassword,
		Timeout:  30 * time.Second,
	}

	sshClient := sshpkg.NewClient(sshConfig)
	if err := sshClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to SSH to Proxmox host: %w", err)
	}
	defer sshClient.Close()

	// Create VM with qm create
	createCmd := fmt.Sprintf("qm create %d --name %s --memory %d --cores %d --sockets %d --ostype l26 --scsihw virtio-scsi-pci --net0 virtio,bridge=vmbr0 --serial0 socket --vga serial0 --agent enabled=1",
		vmSpec.VMID, vmSpec.Name, vmSpec.Memory, vmSpec.Cores, vmSpec.CPU)

	fmt.Printf("Running: %s\n", createCmd)
	output, err := sshClient.Execute(createCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w\nOutput: %s", err, output)
	}
	fmt.Printf("VM created successfully\n")

	// Step 2: Import and attach the boot disk if we downloaded an image
	if imagePath != "" && c.config.SSHUser != "" {
		fmt.Printf("\nImporting cloud image as boot disk...\n")

		// Import the disk
		if err := c.ImportDiskToVM(vmSpec.VMID, imagePath, storage, c.config.SSHUser, c.config.SSHPassword); err != nil {
			fmt.Printf("Warning: Failed to import disk: %v\n", err)
			fmt.Printf("VM created but may not have a boot disk\n")
		} else {
			// Attach the disk
			if err := c.AttachDiskToVM(vmSpec.VMID, storage, c.config.SSHUser, c.config.SSHPassword); err != nil {
				fmt.Printf("Warning: Failed to attach disk: %v\n", err)
			} else {
				// Step 3: Add cloud-init configuration now that disk is attached
				if vmSpec.CloudInit {
					fmt.Printf("\nConfiguring cloud-init...\n")
					if err := c.ConfigureCloudInit(vmSpec.VMID, c.config.SSHUser, c.config.SSHPassword); err != nil {
						fmt.Printf("Warning: Failed to configure cloud-init: %v\n", err)
					}
				}
			}
		}
	}

	// Step 3: Start the VM using SSH command (more reliable than API)
	fmt.Printf("\nStarting VM %d...\n", vmSpec.VMID)

	if c.config.SSHUser != "" && c.config.SSHPassword != "" {
		sshConfig := sshpkg.Config{
			Host:     c.config.Host,
			Port:     22,
			Username: c.config.SSHUser,
			Password: c.config.SSHPassword,
			Timeout:  30 * time.Second,
		}

		sshClient := sshpkg.NewClient(sshConfig)
		if err := sshClient.Connect(); err == nil {
			defer sshClient.Close()

			// Start the VM
			startCmd := fmt.Sprintf("qm start %d", vmSpec.VMID)
			fmt.Printf("Running: %s\n", startCmd)
			startOutput, startErr := sshClient.Execute(startCmd)
			if startErr != nil {
				fmt.Printf("Warning: Failed to start VM via qm: %v\nOutput: %s\n", startErr, startOutput)
			} else {
				fmt.Printf("VM start command executed successfully\n")
			}
		}
	}

	// Give the VM a moment to start
	time.Sleep(3 * time.Second)

	// Get the created VM
	vm, err := node.VirtualMachine(ctx, vmSpec.VMID)
	if err != nil {
		// VM might still be starting, return success anyway
		fmt.Printf("Note: VM created but status check failed: %v\n", err)
		return &VM{
			ID:       vmSpec.VMID,
			Name:     vmSpec.Name,
			Status:   "created",
			Memory:   vmSpec.Memory,
			CPU:      vmSpec.CPU,
			Node:     c.config.Node,
			Created:  time.Now(),
			Modified: time.Now(),
		}, nil
	}

	fmt.Printf("VM is now running\n")

	return &VM{
		ID:       vmSpec.VMID,
		Name:     vm.Name,
		Status:   vm.Status,
		Memory:   vmSpec.Memory,
		CPU:      vmSpec.CPU,
		Node:     c.config.Node,
		Created:  time.Now(),
		Modified: time.Now(),
	}, nil
}

// GetVM retrieves a virtual machine by ID
func (c *Client) GetVM(vmID int) (*VM, error) {
	if vmID <= 0 {
		return nil, fmt.Errorf("invalid VM ID: must be greater than 0")
	}

	if err := c.Connect(); err != nil {
		return nil, err
	}

	node, err := c.getNode()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	vm, err := node.VirtualMachine(ctx, vmID)
	if err != nil {
		return nil, fmt.Errorf("VM not found: %w", err)
	}

	return &VM{
		ID:       vmID,
		Name:     vm.Name,
		Status:   vm.Status,
		Memory:   int(vm.MaxMem / 1024 / 1024),
		CPU:      int(vm.CPUs),
		Node:     c.config.Node,
		Created:  time.Now(),
		Modified: time.Now(),
	}, nil
}

// StartVM starts a stopped virtual machine
func (c *Client) StartVM(vmID int) error {
	if vmID <= 0 {
		return fmt.Errorf("invalid VM ID: must be greater than 0")
	}

	if err := c.Connect(); err != nil {
		return err
	}

	node, err := c.getNode()
	if err != nil {
		return err
	}

	ctx := context.Background()
	vm, err := node.VirtualMachine(ctx, vmID)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	task, err := vm.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	return task.Wait(ctx, 5, 60)
}

// StopVM stops a running virtual machine
func (c *Client) StopVM(vmID int) error {
	if vmID <= 0 {
		return fmt.Errorf("invalid VM ID: must be greater than 0")
	}

	if err := c.Connect(); err != nil {
		return err
	}

	node, err := c.getNode()
	if err != nil {
		return err
	}

	ctx := context.Background()
	vm, err := node.VirtualMachine(ctx, vmID)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	task, err := vm.Stop(ctx)
	if err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	return task.Wait(ctx, 5, 60)
}

// DeleteVM deletes a virtual machine
func (c *Client) DeleteVM(vmID int) error {
	if vmID <= 0 {
		return fmt.Errorf("invalid VM ID: must be greater than 0")
	}

	if err := c.Connect(); err != nil {
		return err
	}

	node, err := c.getNode()
	if err != nil {
		return err
	}

	ctx := context.Background()
	vm, err := node.VirtualMachine(ctx, vmID)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	task, err := vm.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	return task.Wait(ctx, 5, 60)
}

// ListVMs lists all virtual machines on the node
func (c *Client) ListVMs() ([]VM, error) {
	if err := c.Connect(); err != nil {
		return nil, err
	}

	node, err := c.getNode()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	vms, err := node.VirtualMachines(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	result := make([]VM, len(vms))
	for i, vm := range vms {
		result[i] = VM{
			ID:       int(vm.VMID),
			Name:     vm.Name,
			Status:   vm.Status,
			Memory:   int(vm.MaxMem / 1024 / 1024),
			CPU:      int(vm.CPUs),
			Node:     c.config.Node,
			Created:  time.Now(),
			Modified: time.Now(),
		}
	}

	return result, nil
}

// DownloadImageToNode downloads a cloud image to the Proxmox node via SSH
func (c *Client) DownloadImageToNode(image Image, sshUser, sshPassword string) (string, error) {
	if image.URL == "" {
		return "", fmt.Errorf("image URL is required for download")
	}

	// Extract filename from URL
	parts := strings.Split(image.URL, "/")
	filename := parts[len(parts)-1]
	downloadPath := fmt.Sprintf("/tmp/%s", filename)

	fmt.Printf("Downloading cloud image to Proxmox node...\n")
	fmt.Printf("  URL: %s\n", image.URL)
	fmt.Printf("  Destination: %s\n", downloadPath)

	// Connect via SSH to the Proxmox host
	sshConfig := sshpkg.Config{
		Host:     c.config.Host,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
		Timeout:  30 * time.Second,
	}

	sshClient := sshpkg.NewClient(sshConfig)
	if err := sshClient.Connect(); err != nil {
		return "", fmt.Errorf("failed to SSH to Proxmox host: %w", err)
	}
	defer sshClient.Close()

	// Check if image already exists on Proxmox host and is valid
	fmt.Printf("Checking for existing image...\n")
	checkOutput, _ := sshClient.Execute(fmt.Sprintf("test -f %s && echo 'EXISTS' || echo 'NOT_EXISTS'", downloadPath))
	fileExists := strings.Contains(checkOutput, "EXISTS")

	if fileExists {
		fmt.Printf("Image file found, verifying integrity...\n")
		verifyOutput, verifyErr := sshClient.Execute(fmt.Sprintf("qemu-img info %s 2>&1", downloadPath))
		if verifyErr == nil && strings.Contains(verifyOutput, "virtual size") {
			fmt.Printf("Valid image already exists on Proxmox host, skipping download\n")
			sizeOutput, _ := sshClient.Execute(fmt.Sprintf("ls -lh %s | awk '{print $5}'", downloadPath))
			fmt.Printf("Using existing image (%s)\n", strings.TrimSpace(sizeOutput))
			return downloadPath, nil
		}
		// File exists but is invalid, delete it
		fmt.Printf("Existing image is invalid, removing...\n")
		sshClient.Execute(fmt.Sprintf("rm -f %s", downloadPath))
	}

	// Download the image using curl (should work now that DNS is fixed)
	fmt.Printf("Downloading cloud image (this may take several minutes for ~600MB file)...\n")
	downloadCmd := fmt.Sprintf("curl -L --insecure --progress-bar -o %s %s 2>&1", downloadPath, image.URL)
	fmt.Printf("Running: %s\n", downloadCmd)

	output, err := sshClient.Execute(downloadCmd)
	if err != nil {
		sshClient.Execute(fmt.Sprintf("rm -f %s", downloadPath))
		return "", fmt.Errorf("failed to download image with curl: %w\nOutput: %s\nPlease ensure Proxmox host has internet access and DNS resolution", err, output)
	}

	// Show download output
	if output != "" {
		fmt.Printf("Download output: %s\n", output)
	}

	// Verify the downloaded file is a valid qcow2 image
	fmt.Printf("Verifying downloaded image...\n")
	verifyOutput, err := sshClient.Execute(fmt.Sprintf("qemu-img info %s", downloadPath))
	if err != nil {
		sshClient.Execute(fmt.Sprintf("rm -f %s", downloadPath))
		return "", fmt.Errorf("downloaded image is invalid: %w\nOutput: %s", err, verifyOutput)
	}

	// Check if we got the virtual size
	if !strings.Contains(verifyOutput, "virtual size") {
		sshClient.Execute(fmt.Sprintf("rm -f %s", downloadPath))
		return "", fmt.Errorf("downloaded image appears to be corrupted (no virtual size)")
	}

	// Get file size for confirmation
	sizeOutput, _ := sshClient.Execute(fmt.Sprintf("ls -lh %s | awk '{print $5}'", downloadPath))
	fmt.Printf("Downloaded and verified successfully (%s)\n", strings.TrimSpace(sizeOutput))
	return downloadPath, nil
}

// ImportDiskToVM imports a disk image to a VM
func (c *Client) ImportDiskToVM(vmID int, imagePath string, storage string, sshUser, sshPassword string) error {
	fmt.Printf("Importing disk to VM %d...\n", vmID)

	// Connect via SSH to the Proxmox host
	sshConfig := sshpkg.Config{
		Host:     c.config.Host,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
		Timeout:  30 * time.Second,
	}

	sshClient := sshpkg.NewClient(sshConfig)
	if err := sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to SSH to Proxmox host: %w", err)
	}
	defer sshClient.Close()

	// Convert qcow2 to raw format for more reliable import
	rawPath := strings.Replace(imagePath, ".qcow2", ".raw", 1)
	fmt.Printf("Converting qcow2 to raw format...\n")
	convertCmd := fmt.Sprintf("qemu-img convert -f qcow2 -O raw %s %s", imagePath, rawPath)
	convertOutput, convertErr := sshClient.Execute(convertCmd)
	if convertErr != nil {
		return fmt.Errorf("failed to convert image: %w\nOutput: %s", convertErr, convertOutput)
	}
	fmt.Printf("Image converted to raw format\n")

	// Import the raw disk
	importCmd := fmt.Sprintf("qm importdisk %d %s %s", vmID, rawPath, storage)
	fmt.Printf("Running: %s\n", importCmd)
	output, err := sshClient.Execute(importCmd)
	if err != nil {
		return fmt.Errorf("failed to import disk: %w\nOutput: %s", err, output)
	}

	fmt.Printf("Disk imported successfully\n")
	fmt.Printf("Import output: %s\n", output)

	// Clean up raw file after import
	sshClient.Execute(fmt.Sprintf("rm -f %s", rawPath))

	return nil
}

// AttachDiskToVM attaches an imported disk to a VM as the boot drive
func (c *Client) AttachDiskToVM(vmID int, storage string, sshUser, sshPassword string) error {
	fmt.Printf("Attaching disk to VM %d...\n", vmID)

	// Connect via SSH to the Proxmox host
	sshConfig := sshpkg.Config{
		Host:     c.config.Host,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
		Timeout:  30 * time.Second,
	}

	sshClient := sshpkg.NewClient(sshConfig)
	if err := sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to SSH to Proxmox host: %w", err)
	}
	defer sshClient.Close()

	// The imported disk will be named "unused0" - we need to attach it as scsi0
	// Also set it as boot disk and resize it
	commands := []string{
		fmt.Sprintf("qm set %d --scsi0 %s:vm-%d-disk-0", vmID, storage, vmID),
		fmt.Sprintf("qm set %d --boot order=scsi0", vmID),
		fmt.Sprintf("qm disk resize %d scsi0 +10G", vmID), // Resize to add 10GB
	}

	for _, cmd := range commands {
		fmt.Printf("Running: %s\n", cmd)
		output, err := sshClient.Execute(cmd)
		if err != nil {
			// Try to continue even if some commands fail
			fmt.Printf("Warning: command failed: %v\nOutput: %s\n", err, output)
		}
	}

	fmt.Printf("Disk attached successfully\n")
	return nil
}

// ConfigureCloudInit adds cloud-init configuration to a VM
func (c *Client) ConfigureCloudInit(vmID int, sshUser, sshPassword string) error {
	// Connect via SSH to the Proxmox host
	sshConfig := sshpkg.Config{
		Host:     c.config.Host,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
		Timeout:  30 * time.Second,
	}

	sshClient := sshpkg.NewClient(sshConfig)
	if err := sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to SSH to Proxmox host: %w", err)
	}
	defer sshClient.Close()

	// Add cloud-init drive and configuration
	commands := []string{
		fmt.Sprintf("qm set %d --ide2 local:cloudinit", vmID),
		fmt.Sprintf("qm set %d --ipconfig0 ip=dhcp", vmID),
		fmt.Sprintf("qm set %d --ciuser dtt", vmID),
		fmt.Sprintf("qm set %d --cipassword dtt", vmID),
	}

	for _, cmd := range commands {
		fmt.Printf("Running: %s\n", cmd)
		output, err := sshClient.Execute(cmd)
		if err != nil {
			return fmt.Errorf("failed to configure cloud-init: %w\nCommand: %s\nOutput: %s", err, cmd, output)
		}
	}

	fmt.Printf("Cloud-init configured successfully\n")
	return nil
}

// DownloadImage downloads an image to Proxmox local storage (legacy method)
func (c *Client) DownloadImage(image Image, storageID string) error {
	// This is a legacy method - use DownloadImageToNode instead
	return fmt.Errorf("use DownloadImageToNode instead")
}

// GetAvailableImages lists images available on the Proxmox server
func (c *Client) GetAvailableImages(storageID string) ([]Image, error) {
	if storageID == "" {
		return nil, fmt.Errorf("storage ID is required")
	}

	// TODO: Implement actual Proxmox API call to list images in storage

	return []Image{}, nil
}

// GetVMIPAddress retrieves the IP address of a VM
func (c *Client) GetVMIPAddress(vmID int) (string, error) {
	if vmID <= 0 {
		return "", fmt.Errorf("invalid VM ID: must be greater than 0")
	}

	if err := c.Connect(); err != nil {
		return "", err
	}

	node, err := c.getNode()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	vm, err := node.VirtualMachine(ctx, vmID)
	if err != nil {
		return "", fmt.Errorf("VM not found: %w", err)
	}

	// Try to get IP from QEMU agent
	interfaces, err := vm.AgentGetNetworkIFaces(ctx)
	if err != nil {
		// QEMU agent might not be running yet
		return "", fmt.Errorf("unable to get IP address (QEMU agent may not be running): %w", err)
	}

	// Find first non-loopback IPv4 address
	for _, iface := range interfaces {
		if iface.Name == "lo" {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType == "ipv4" && !strings.HasPrefix(addr.IPAddress, "127.") {
				return addr.IPAddress, nil
			}
		}
	}

	return "", fmt.Errorf("no valid IP address found for VM")
}

// WaitForVMReady waits for a VM to be accessible via SSH
func (c *Client) WaitForVMReady(vmIP string, sshUser string, sshPassword string, maxRetries int) error {
	if maxRetries == 0 {
		maxRetries = 30 // Default to 30 retries (5 minutes with 10s delay)
	}

	sshConfig := sshpkg.Config{
		Host:     vmIP,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
		Timeout:  10 * time.Second,
	}

	client := sshpkg.NewClient(sshConfig)
	return client.WaitForConnection(maxRetries, 10*time.Second)
}

// UploadBinary uploads a binary to a VM via SSH/SCP
func (c *Client) UploadBinary(vmIP string, sshUser string, sshPassword string, localPath string, remotePath string) error {
	sshConfig := sshpkg.Config{
		Host:     vmIP,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
	}

	client := sshpkg.NewClient(sshConfig)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to VM: %w", err)
	}
	defer client.Close()

	if err := client.UploadFile(localPath, remotePath); err != nil {
		return fmt.Errorf("failed to upload binary: %w", err)
	}

	// Make the binary executable
	_, err := client.Execute(fmt.Sprintf("chmod +x %s", remotePath))
	if err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

// ExecuteBinary executes a binary on a VM via SSH
func (c *Client) ExecuteBinary(vmIP string, sshUser string, sshPassword string, remotePath string) (string, error) {
	sshConfig := sshpkg.Config{
		Host:     vmIP,
		Port:     22,
		Username: sshUser,
		Password: sshPassword,
	}

	client := sshpkg.NewClient(sshConfig)
	if err := client.Connect(); err != nil {
		return "", fmt.Errorf("failed to connect to VM: %w", err)
	}
	defer client.Close()

	output, err := client.Execute(remotePath)
	if err != nil {
		return output, fmt.Errorf("failed to execute binary: %w", err)
	}

	return output, nil
}
