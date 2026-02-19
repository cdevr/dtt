package main

import (
	"context"
	"fmt"
	"time"

	"github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	vmRmCommand = &cobra.Command{
		Use:   "rm <name-or-id>",
		Short: "remove vm",
		Args:  cobra.MinimumNArgs(1),
		RunE:  command_vm_rm,
	}
)

func init() {
	vmCommand.AddCommand(vmRmCommand)
}

func command_vm_rm(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	pac := getPACFromFlags()

	cluster, err := pac.Cluster(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster gave err: %w", err)
	}

	resources, err := cluster.Resources(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster resources gave err: %w", err)
	}

	toDelete := []*proxmox.ClusterResource{}

	for _, query := range args {
		found := false
		for _, r := range resources {
			if r.Type != "qemu" {
				continue
			}

			match := false
			if fmt.Sprintf("%d", r.VMID) == query {
				match = true
			}
			if r.Name == query {
				match = true
			}
			if !match {
				continue
			}
			found = true

			toDelete = append(toDelete, r)
		}
		if !found {
			return fmt.Errorf("failed to find VM for query %q", query)
		}
	}

	tasks := []*proxmox.Task{}
	for _, r := range toDelete {
		node, err := pac.Node(ctx, r.Node)
		if err != nil {
			return fmt.Errorf("failed to get the node to for nodename %q: %s", r.Node, err)
		}
		vm, err := node.VirtualMachine(ctx, int(r.VMID))
		if err != nil {
			return fmt.Errorf("failed to get the virtual machine for VMID %q: %w", r.VMID, err)
		}

		deleteTask, err := vm.Delete(ctx)
		if err != nil {
			return fmt.Errorf("failed to start delete task for machine VMID %q: %w", r.VMID, err)
		}
		tasks = append(tasks, deleteTask)
	}

	for _, task := range tasks {
		if err := task.Wait(ctx, time.Second, 2*time.Minute); err != nil {
			return fmt.Errorf("waiting for delete task failed: %w", err)
		}
	}
	return nil
}