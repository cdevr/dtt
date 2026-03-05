package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	imageTemplateCommand = &cobra.Command{
		Use:   "download-template <release>",
		Short: "download a standard cloud image template (e.g., ubuntu:24.04, debian:bookworm)",
		Long: `Download a standard cloud image template to Proxmox storage.

Supported releases:
  Ubuntu: ubuntu:noble, ubuntu:jammy, ubuntu:focal, ubuntu:bionic, ubuntu:xenial
          or ubuntu:24.04, ubuntu:22.04, ubuntu:20.04, ubuntu:18.04, ubuntu:16.04
  Debian: debian:trixie, debian:bookworm, debian:bullseye, debian:buster
          or debian:13, debian:12, debian:11, debian:10

Examples:
  dtt image download-template ubuntu:24.04
  dtt image download-template debian:bookworm
  dtt image download-template ubuntu:noble --storage local-lvm`,
		Args: cobra.ExactArgs(1),
		RunE: command_image_template,
	}

	imageListTemplatesCommand = &cobra.Command{
		Use:   "list-templates",
		Short: "list available cloud image templates that can be downloaded",
		RunE:  command_image_list_templates,
	}

	FlagImageTemplateNode    *string
	FlagImageTemplateStorage *string
)

func init() {
	FlagImageTemplateNode = imageTemplateCommand.PersistentFlags().String("node", "pve", "which node to download the image to")
	FlagImageTemplateStorage = imageTemplateCommand.PersistentFlags().String("storage", "local", "which storage to download the image to")

	imageCommand.AddCommand(imageTemplateCommand)
	imageCommand.AddCommand(imageListTemplatesCommand)
}

func command_image_template(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	pac := getPACFromFlags()

	release := strings.TrimSpace(args[0])
	if release == "" {
		return fmt.Errorf("release cannot be empty")
	}

	distro, version, err := extractDistroVersionFromRelease(release)
	if err != nil {
		return err
	}

	if distro == "" {
		return fmt.Errorf("invalid release format %q, expected format: distro:version (e.g., ubuntu:24.04)", release)
	}

	cloudImageURL, err := getFnFromCloudImageURL(distro, version, release)
	if err != nil {
		return fmt.Errorf("failed to get cloud image URL: %w", err)
	}

	qcow2Name, err := extractFn(cloudImageURL)
	if err != nil {
		return fmt.Errorf("failed to extract filename from URL %q", cloudImageURL)
	}

	// Needed for ubuntu minimal cloud images (they use .img extension but are qcow2)
	qcow2Name = strings.ReplaceAll(qcow2Name, ".img", ".qcow2")

	node, err := pac.Node(ctx, *FlagImageTemplateNode)
	if err != nil {
		return fmt.Errorf("getting node %s: %w", *FlagImageTemplateNode, err)
	}

	storage, err := node.Storage(ctx, *FlagImageTemplateStorage)
	if err != nil {
		return fmt.Errorf("getting storage %s on node %s: %w", *FlagImageTemplateStorage, *FlagImageTemplateNode, err)
	}

	// Check if image already exists
	content, err := storage.GetContent(ctx)
	if err != nil {
		return fmt.Errorf("getting storage content: %w", err)
	}

	expectedVolid := fmt.Sprintf("%s:import/%s", *FlagImageTemplateStorage, qcow2Name)
	for _, c := range content {
		if c.Volid == expectedVolid {
			fmt.Printf("image %s already exists at %s\n", qcow2Name, expectedVolid)
			return nil
		}
	}

	fmt.Printf("downloading %s (%s) to %s/%s...\n", release, qcow2Name, *FlagImageTemplateNode, *FlagImageTemplateStorage)
	fmt.Printf("source: %s\n", cloudImageURL)

	task, err := storage.DownloadURL(ctx, "import", qcow2Name, cloudImageURL)
	if err != nil {
		return fmt.Errorf("downloading image: %w", err)
	}

	if err := task.Wait(ctx, time.Second, 30*time.Minute); err != nil {
		return fmt.Errorf("waiting for download: %w", err)
	}

	fmt.Printf("downloaded %s to %s:import/%s\n", release, *FlagImageTemplateStorage, qcow2Name)
	return nil
}

func command_image_list_templates(cmd *cobra.Command, args []string) error {
	fmt.Println("Available cloud image templates:")
	fmt.Println()
	fmt.Println("Ubuntu (minimal cloud images):")
	fmt.Println("  ubuntu:noble   (24.04 LTS)")
	fmt.Println("  ubuntu:jammy   (22.04 LTS)")
	fmt.Println("  ubuntu:focal   (20.04 LTS)")
	fmt.Println("  ubuntu:bionic  (18.04 LTS)")
	fmt.Println("  ubuntu:xenial  (16.04 LTS)")
	fmt.Println()
	fmt.Println("Debian (generic cloud images):")
	fmt.Println("  debian:trixie    (13)")
	fmt.Println("  debian:bookworm  (12)")
	fmt.Println("  debian:bullseye  (11)")
	fmt.Println("  debian:buster    (10)")
	fmt.Println()
	fmt.Println("You can also use version numbers: ubuntu:24.04, debian:12, etc.")
	fmt.Println()
	fmt.Println("Example: dtt image download-template ubuntu:noble")
	return nil
}
