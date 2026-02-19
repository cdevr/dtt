package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	px "github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "dtt",
		Short: "DTT - Do The Thing: Run all sorts of stuff on Proxmox VMs. Linux binaries, docker images",
		Long: `DTT is a CLI tool that helps you run Linux binaries, docker images on Proxmox VE.
It handles image download, VM creation, cloud-init configuration, and binary execution.`,
	}

	FlagHost         = rootCmd.PersistentFlags().String("proxmox-host", "", "Proxmox server hostname or IP")
	FlagPort         = rootCmd.PersistentFlags().Int("proxmox-port", 8006, "Proxmox server port")
	FlagUserName     = rootCmd.PersistentFlags().String("proxmox-user", "", "Proxmox API username")
	FlagUserPassword = rootCmd.PersistentFlags().String("proxmox-password", "", "Proxmox API password (or set DTT_PROXMOX_PASSWORD, encouraged, or better yet use tokens)")
	FlagTokenID      = rootCmd.PersistentFlags().String("proxmox-token-id", "", "Proxmox API Token ID")
	FlagTokenSecret  = rootCmd.PersistentFlags().String("proxmox-token-secret", "", "Proxmox API Token secret")
	FlagInsecure     = rootCmd.PersistentFlags().Bool("proxmox-insecure", true, "Skip SSL certificate verification")

	vmCommand = &cobra.Command{
		Use:   "vm",
		Short: "vm commands",
	}

	imageCommand = &cobra.Command{
		Use:   "image",
		Short: "image commands",
	}

	agentCommand = &cobra.Command{
		Use:   "agent",
		Short: "qemu agent commands",
	}
)

func getPACFromFlags() *px.Client {
	HTTPClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: *FlagInsecure,
			},
		},
	}

	opts := []px.Option{
		px.WithHTTPClient(&HTTPClient),
	}
	if *FlagTokenID != "" {
		opts = append(opts, px.WithAPIToken(*FlagTokenID, *FlagTokenSecret))
	}
	if *FlagUserName != "" {
		opts = append(opts, px.WithCredentials(&px.Credentials{
			Username: *FlagUserName,
			Password: *FlagUserPassword,
		}))
	}

	url := fmt.Sprintf("https://%s:%d/api2/json", *FlagHost, *FlagPort)
	client := px.NewClient(url, opts...)

	return client
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(vmCommand)
	rootCmd.AddCommand(imageCommand)
	rootCmd.AddCommand(agentCommand)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}