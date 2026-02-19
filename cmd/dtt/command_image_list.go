package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	imageListCommand = &cobra.Command{
		Use:   "list",
		Short: "list available VM images",
		RunE:  command_image_list,
	}

	FlagImageListNode    *string
	FlagImageListStorage *string
)

func init() {
	FlagImageListNode = imageListCommand.PersistentFlags().String("node", "pve", "which node to list images from")
	FlagImageListStorage = imageListCommand.PersistentFlags().String("storage", "local", "which storage to list images from")
	imageCommand.AddCommand(imageListCommand)
}

func command_image_list(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	pac := getPACFromFlags()

	node, err := pac.Node(ctx, *FlagImageListNode)
	if err != nil {
		return fmt.Errorf("getting node %s gave err: %w", *FlagImageListNode, err)
	}

	storage, err := node.Storage(ctx, *FlagImageListStorage)
	if err != nil {
		return fmt.Errorf("getting storage %s on node %s gave err: %w", *FlagImageListStorage, *FlagImageListNode, err)
	}

	content, err := storage.GetContent(ctx)
	if err != nil {
		return fmt.Errorf("getting storage content gave err: %w", err)
	}

	imageRows := make([]struct {
		Name   string
		Format string
		Size   uint64
		VolID  string
	}, 0, len(content))

	prefix := *FlagImageListStorage + ":import/"
	for _, c := range content {
		if !strings.Contains(c.Volid, ":import/") {
			continue
		}

		name := strings.TrimPrefix(c.Volid, prefix)
		if name == c.Volid {
			if idx := strings.LastIndex(c.Volid, "/"); idx >= 0 && idx+1 < len(c.Volid) {
				name = c.Volid[idx+1:]
			}
		}

		imageRows = append(imageRows, struct {
			Name   string
			Format string
			Size   uint64
			VolID  string
		}{
			Name:   name,
			Format: c.Format,
			Size:   c.Size,
			VolID:  c.Volid,
		})
	}

	sort.Slice(imageRows, func(i, j int) bool {
		if imageRows[i].Name == imageRows[j].Name {
			return imageRows[i].VolID < imageRows[j].VolID
		}
		return imageRows[i].Name < imageRows[j].Name
	})

	fmt.Printf("Images on %s/%s\n", *FlagImageListNode, *FlagImageListStorage)
	if len(imageRows) == 0 {
		fmt.Println("No import images found.")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tFORMAT\tSIZE\tVOLID")
	for _, row := range imageRows {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", row.Name, row.Format, formatBytes(row.Size), row.VolID)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing image list writer gave err: %w", err)
	}

	return nil
}