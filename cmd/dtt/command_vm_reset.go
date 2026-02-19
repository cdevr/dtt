package main

import (
	"context"
	"fmt"
	"time"

	"github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	vmResetCommand = &cobra.Command{
		Use:   "reset <name-or-id>",
		Short: "reset vm",
		Args:  cobra.MinimumNArgs(1),
		RunE:  command_vm_reset,
	}
)

func init() {
	vmCommand.AddCommand(vmResetCommand)
}

func command_vm_reset(cmd *cobra.Command, args []string) error {
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

	toReset := []*proxmox.ClusterResource{}

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

			toReset = append(toReset, r)
		}
		if !found {
			return fmt.Errorf("failed to find VM for query %q", query)
		}
	}

	tasks := []*proxmox.Task{}
	for _, r := range toReset {
		node, err := pac.Node(ctx, r.Node)
		if err != nil {
			return fmt.Errorf("failed to get the node to for nodename %q: %s", r.Node, err)
		}
		vm, err := node.VirtualMachine(ctx, int(r.VMID))
		if err != nil {
			return fmt.Errorf("failed to get the virtual machine for VMID %q: %w", r.VMID, err)
		}

		resetTask, err := vm.Reset(ctx)
		if err != nil {
			return fmt.Errorf("failed to start reset task for machine VMID %q: %w", r.VMID, err)
		}
		tasks = append(tasks, resetTask)
	}

	for _, task := range tasks {
		if err := task.Wait(ctx, time.Second, 2*time.Minute); err != nil {
			return fmt.Errorf("waiting for reset task failed: %w", err)
		}
	}
	return nil
}