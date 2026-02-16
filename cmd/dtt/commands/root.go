package commands

import (
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root command
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "dtt",
		Short: "DTT - Do The Thing: Run Linux binaries on Proxmox VMs",
		Long: `DTT is a CLI tool that helps you run Linux binaries on Proxmox virtual machines.
It handles image download, VM creation, cloud-init configuration, and binary execution.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().String("proxmox-host", "localhost", "Proxmox server hostname or IP")
	rootCmd.PersistentFlags().Int("proxmox-port", 8006, "Proxmox server port")
	rootCmd.PersistentFlags().String("proxmox-user", "root@pam", "Proxmox API username")
	rootCmd.PersistentFlags().String("proxmox-password", "", "Proxmox API password (or set DTT_PROXMOX_PASSWORD)")
	rootCmd.PersistentFlags().String("proxmox-node", "pve", "Proxmox node name")
	rootCmd.PersistentFlags().Bool("proxmox-insecure", false, "Skip SSL certificate verification")
	rootCmd.PersistentFlags().String("proxmox-ssh-user", "root", "Proxmox host SSH username (or set DTT_PROXMOX_SSH_USER)")
	rootCmd.PersistentFlags().String("proxmox-ssh-password", "", "Proxmox host SSH password (or set DTT_PROXMOX_SSH_PASSWORD)")
	rootCmd.PersistentFlags().Int("proxmox-ssh-port", 22, "Proxmox host SSH port")

	// Add subcommands
	rootCmd.AddCommand(NewRunCommand())
	rootCmd.AddCommand(NewImageCommand())
	rootCmd.AddCommand(NewVMCommand())
	rootCmd.AddCommand(NewCompletionCommand(rootCmd))

	return rootCmd
}
