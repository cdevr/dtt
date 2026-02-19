package main

import (
	"context"
	"fmt"

	"time"

	"github.com/spf13/cobra"
)

var (
	imageUploadCommand = &cobra.Command{
		Use:   "upload",
		Short: "upload a VM image",
		Args:  cobra.ExactArgs(1),
		RunE:  command_image_upload,
	}

	FlagImageUploadNode    *string
	FlagImageUploadStorage *string
)

func init() {
	FlagImageUploadNode = imageUploadCommand.PersistentFlags().String("node", "pve", "which node to upload the image to")
	FlagImageUploadStorage = imageUploadCommand.PersistentFlags().String("storage", "local", "which storage to upload the image to")

	imageCommand.AddCommand(imageUploadCommand)
}

func command_image_upload(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	pac := getPACFromFlags()

	if len(args) != 1 {
		return fmt.Errorf("usage: dtt image upload <local-image-file>")
	}

	imageFile := args[0]

	node, err := pac.Node(ctx, *FlagImageUploadNode)
	if err != nil {
		return fmt.Errorf("getting node %s gave err: %w", *FlagImageUploadNode, err)
	}

	storage, err := node.Storage(ctx, *FlagImageUploadStorage)
	if err != nil {
		return fmt.Errorf("getting storage %s on node %s gave err: %w", *FlagImageUploadStorage, *FlagImageUploadNode, err)
	}

	fmt.Printf("uploading image %s to %s/%s\n", imageFile, *FlagImageUploadNode, *FlagImageUploadStorage)
	task, err := storage.Upload("import", imageFile)
	if err != nil {
		return fmt.Errorf("uploading image %s to %s/%s gave err: %w", imageFile, *FlagImageUploadNode, *FlagImageUploadStorage, err)
	}

	if err := task.Wait(ctx, time.Second, 30*time.Minute); err != nil {
		return fmt.Errorf("waiting for upload task gave err: %w", err)
	}

	fmt.Printf("uploaded image %s to %s/%s\n", imageFile, *FlagImageUploadNode, *FlagImageUploadStorage)
	return nil
}