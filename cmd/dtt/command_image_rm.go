package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	imageRmCommand = &cobra.Command{
		Use:   "rm",
		Short: "remove a VM image from storage",
		Args:  cobra.ExactArgs(1),
		RunE:  command_image_rm,
	}

	FlagImageRmNode    *string
	FlagImageRmStorage *string
)

func init() {
	FlagImageRmNode = imageRmCommand.PersistentFlags().String("node", "pve", "which node the image is on")
	FlagImageRmStorage = imageRmCommand.PersistentFlags().String("storage", "local", "which storage the image is on")

	imageCommand.AddCommand(imageRmCommand)
}

func command_image_rm(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	pac := getPACFromFlags()

	if len(args) != 1 {
		return fmt.Errorf("usage: dtt image rm <image-name>")
	}

	imageName := args[0]

	node, err := pac.Node(ctx, *FlagImageRmNode)
	if err != nil {
		return fmt.Errorf("getting node %s gave err: %w", *FlagImageRmNode, err)
	}

	storage, err := node.Storage(ctx, *FlagImageRmStorage)
	if err != nil {
		return fmt.Errorf("getting storage %s on node %s gave err: %w", *FlagImageRmStorage, *FlagImageRmNode, err)
	}

	volid := fmt.Sprintf("%s:import/%s", *FlagImageRmStorage, imageName)
	fmt.Printf("removing image %s from %s/%s\n", imageName, *FlagImageRmNode, *FlagImageRmStorage)

	task, err := storage.DeleteContent(ctx, volid)
	if err != nil {
		return fmt.Errorf("deleting image %s gave err: %w", volid, err)
	}

	if err := task.Wait(ctx, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for delete task gave err: %w", err)
	}

	fmt.Printf("removed image %s from %s/%s\n", imageName, *FlagImageRmNode, *FlagImageRmStorage)
	return nil
}
