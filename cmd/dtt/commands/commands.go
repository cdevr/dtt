package commands

import (
	"fmt"
	"os"

	"github.com/example/dtt/pkg/proxmox"
	"github.com/spf13/cobra"
)

// NewRunCommand creates the run subcommand
func NewRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <binary> <vm-id>",
		Short: "Run a Linux binary on a Proxmox VM",
		Long: `Run a Linux binary on a Proxmox VM. The binary will be uploaded and executed.
This command handles image provisioning, VM creation, and binary execution.`,
		Args: cobra.MinimumNArgs(2),
		RunE: command_run,
	}

	// Add flags specific to run command
	cmd.Flags().String("hostname", "dtt-vm", "VM hostname")
	cmd.Flags().String("image", "debian-11", "Image to use (debian-11, debian-13, ubuntu-24.04)")
	cmd.Flags().Int("memory", 512, "Memory in MB")
	cmd.Flags().Int("cpu", 1, "Number of CPUs")
	cmd.Flags().Int("cores", 1, "Cores per CPU")
	cmd.Flags().String("username", "dtt", "Default username")
	cmd.Flags().String("remote-path", "/tmp/binary", "Path to place binary on VM")
	cmd.Flags().String("vm-ip", "", "VM IP address for SSH connection")
	cmd.Flags().String("ssh-password", "", "SSH password (or set DTT_SSH_PASSWORD)")

	return cmd
}

// NewImageCommand creates the image subcommand
func NewImageCommand() *cobra.Command {
	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Manage VM images",
		Long:  "Manage VM images for use with DTT",
	}

	imageCmd.AddCommand(NewImageListCommand())
	imageCmd.AddCommand(NewImageDownloadCommand())

	return imageCmd
}

// NewImageListCommand creates the image list subcommand
func NewImageListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available images",
		Long:  "List available images that can be used for VM creation",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Available images:")
			fmt.Println("ID                Size")
			fmt.Println("---               ----")

			for _, img := range proxmox.DefaultImages() {
				size := "Unknown"
				if img.Size > 0 {
					size = fmt.Sprintf("%d MB", img.Size/1024/1024)
				}
				fmt.Printf("%-17s %s\n", img.Name, size)
			}

			return nil
		},
	}
}

// NewImageDownloadCommand creates the image download subcommand
func NewImageDownloadCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "download <image-name>",
		Short: "Download an image to Proxmox storage",
		Long:  "Download a VM image to Proxmox local storage for faster provisioning",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageName := args[0]
			storage, _ := cmd.Flags().GetString("storage")

			client := getProxmoxClient(cmd)

			// Find the image
			var selectedImage proxmox.Image
			found := false
			for _, img := range proxmox.DefaultImages() {
				if img.Name == imageName {
					selectedImage = img
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("image not found: %s", imageName)
			}

			fmt.Printf("Downloading image: %s\n", selectedImage.Name)
			fmt.Printf("Source: %s\n", selectedImage.URL)

			if err := client.DownloadImage(selectedImage, storage); err != nil {
				return fmt.Errorf("failed to download image: %w", err)
			}

			fmt.Printf("Image downloaded successfully\n")
			return nil
		},
	}
}

// NewVMCommand creates the vm subcommand
func NewVMCommand() *cobra.Command {
	vmCmd := &cobra.Command{
		Use:   "vm",
		Short: "Manage virtual machines",
		Long:  "Manage virtual machines on Proxmox",
	}

	vmCmd.AddCommand(NewVMListCommand())
	vmCmd.AddCommand(NewVMDeleteCommand())

	return vmCmd
}

// NewVMListCommand creates the vm list subcommand
func NewVMListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List VMs",
		Long:  "List all virtual machines on the Proxmox node",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getProxmoxClient(cmd)

			vms, err := client.ListVMs()
			if err != nil {
				return fmt.Errorf("failed to list VMs: %w", err)
			}

			fmt.Println("Virtual Machines:")
			fmt.Println("ID   Name                      Status    Memory  CPU")
			fmt.Println("--   ----                      ------    ------  ---")

			for _, vm := range vms {
				fmt.Printf("%-4d %-25s %-9s %-7d %d\n", vm.ID, vm.Name, vm.Status, vm.Memory, vm.CPU)
			}

			return nil
		},
	}
}

// NewVMDeleteCommand creates the vm delete subcommand
func NewVMDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <vm-id>",
		Short: "Delete a VM",
		Long:  "Delete a virtual machine from Proxmox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := parseVMID(args[0])
			client := getProxmoxClient(cmd)

			fmt.Printf("Deleting VM %d...\n", vmID)

			if err := client.DeleteVM(vmID); err != nil {
				return fmt.Errorf("failed to delete VM: %w", err)
			}

			fmt.Printf("VM %d deleted successfully\n", vmID)
			return nil
		},
	}
}

// NewCompletionCommand creates the completion subcommand for bash/zsh
func NewCompletionCommand(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate a shell completion script for DTT. 
To load completions in bash:
  source <(dtt completion bash)
  
To load completions in zsh:
  source <(dtt completion zsh)`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletion(os.Stdout)
			}
			return nil
		},
	}
}

// Helper functions

func getProxmoxClient(cmd *cobra.Command) *proxmox.Client {
	host, _ := cmd.Flags().GetString("proxmox-host")
	port, _ := cmd.Flags().GetInt("proxmox-port")
	username, _ := cmd.Flags().GetString("proxmox-user")
	password, _ := cmd.Flags().GetString("proxmox-password")
	node, _ := cmd.Flags().GetString("proxmox-node")
	insecure, _ := cmd.Flags().GetBool("proxmox-insecure")
	sshUser, _ := cmd.Flags().GetString("proxmox-ssh-user")
	sshPassword, _ := cmd.Flags().GetString("proxmox-ssh-password")
	sshPort, _ := cmd.Flags().GetInt("proxmox-ssh-port")

	// Check environment variables for authentication
	if password == "" {
		password = os.Getenv("DTT_PROXMOX_PASSWORD")
	}

	tokenID := os.Getenv("DTT_PROXMOX_TOKEN_ID")
	tokenSecret := os.Getenv("DTT_PROXMOX_TOKEN_SECRET")

	// Check environment variables for SSH credentials
	if sshUser == "" || sshUser == "root" {
		if envSSHUser := os.Getenv("DTT_PROXMOX_SSH_USER"); envSSHUser != "" {
			sshUser = envSSHUser
		}
	}
	if sshPassword == "" {
		sshPassword = os.Getenv("DTT_PROXMOX_SSH_PASSWORD")
	}

	config := proxmox.ClientConfig{
		Host:        host,
		Port:        port,
		Username:    username,
		Password:    password,
		TokenID:     tokenID,
		TokenSecret: tokenSecret,
		Node:        node,
		Insecure:    insecure,
		SSHUser:     sshUser,
		SSHPassword: sshPassword,
		SSHPort:     sshPort,
	}

	return proxmox.NewClient(config)
}

func selectImage(imageID string) (proxmox.Image, error) {
	images := proxmox.DefaultImages()

	// Map common names to images
	imageMap := map[string]proxmox.Image{
		"debian-11":    images[0],
		"debian-13":    images[1],
		"ubuntu-24.04": images[2],
		"ubuntu":       images[2],
		"debian":       images[0],
	}

	if img, ok := imageMap[imageID]; ok {
		return img, nil
	}

	// Default to Debian 11
	return images[0], nil
}

func parseVMID(vmIDStr string) int {
	var vmID int
	fmt.Sscanf(vmIDStr, "%d", &vmID)
	return vmID
}
