package main

import (
	"context"
	"fmt"
	"log"
	"sync"
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

	FlagVmRmStop *bool
)

func init() {
	vmCommand.AddCommand(vmRmCommand)

	FlagVmRmStop = vmRmCommand.PersistentFlags().Bool("stop", false, "stop VMs before removing them")
}

var (
	nodeCache = map[string]*proxmox.Node{}
	vmCache   = map[string]*proxmox.VirtualMachine{}
)

func WaitOnManyTasks(ctx context.Context, tasks []*proxmox.Task, pollInterval time.Duration, timeout time.Duration) error {
	if len(tasks) == 0 {
		return nil
	}

	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup
	wg.Add(len(tasks))

	for _, task := range tasks {
		task := task
		go func() {
			defer wg.Done()
			if err := task.Wait(ctx, pollInterval, timeout); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	if err, ok := <-errCh; ok {
		return err
	}

	return nil
}

func getNodeCached(ctx context.Context, pac *proxmox.Client, node string) (*proxmox.Node, error) {
	if node, ok := nodeCache[node]; ok {
		return node, nil
	}
	result, err := pac.Node(ctx, node)
	if err != nil {
		return nil, err
	}
	nodeCache[node] = result
	return result, nil
}

func getVMCached(ctx context.Context, node *proxmox.Node, vmid int) (*proxmox.VirtualMachine, error) {
	key := fmt.Sprintf("%s:%d", node.Name, vmid)
	if vm, ok := vmCache[key]; ok {
		return vm, nil
	}

	result, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		return nil, err
	}

	vmCache[key] = result
	return result, nil
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
		node, err := getNodeCached(ctx, pac, r.Node)
		if err != nil {
			return fmt.Errorf("failed to get the node to for nodename %q: %s", r.Node, err)
		}
		vm, err := getVMCached(ctx, node, int(r.VMID))
		if err != nil {
			return fmt.Errorf("failed to get the virtual machine for VMID %q: %w", r.VMID, err)
		}

		if !vm.IsStopped() {
			if *FlagVmRmStop {
				log.Printf("Warning: VM %q (ID %d) is not stopped, adding stop task", vm.Name, vm.VMID)
				stopTask, err := vm.Stop(ctx)
				if err != nil {
					return fmt.Errorf("Error creating stop task for VM %q (ID %d): %w", vm.Name, vm.VMID, err)
				}
				tasks = append(tasks, stopTask)
			} else {
				log.Printf("Warning: VM %q (ID %d) is not stopped", vm.Name, vm.VMID)
			}
		}
	}

	if err := WaitOnManyTasks(ctx, tasks, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for delete task failed: %w", err)
	}

	for _, r := range toDelete {
		node, err := getNodeCached(ctx, pac, r.Node)
		if err != nil {
			return fmt.Errorf("failed to get the node to for nodename %q: %s", r.Node, err)
		}
		vm, err := getVMCached(ctx, node, int(r.VMID))
		if err != nil {
			return fmt.Errorf("failed to get the virtual machine for VMID %q: %w", r.VMID, err)
		}

		deleteTask, err := vm.Delete(ctx)
		if err != nil {
			return fmt.Errorf("failed to start delete task for machine VMID %q: %w", r.VMID, err)
		}
		tasks = append(tasks, deleteTask)
	}

	if err := WaitOnManyTasks(ctx, tasks, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for delete task failed: %w", err)
	}

	return nil
}
