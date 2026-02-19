package main

import (
	"context"
	"fmt"
	"time"

	"github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	vmStartCommand = &cobra.Command{
		Use:   "start",
		Short: "start a new vm",
		RunE:  command_vm_start,
	}

	FlagVmStartNode   *string
	FlagVmStartName   *string
	FlagVmStartMemory *int
	FlagVmStartCores  *int
)

func init() {
	vmCommand.AddCommand(vmStartCommand)

	FlagVmStartNode = vmStartCommand.PersistentFlags().String("node", "pve", "which node to start the vm on")
	FlagVmStartName = vmStartCommand.PersistentFlags().String("name", "", "name of vm to create (default: dtt-vm-<id>)")
	FlagVmStartMemory = vmStartCommand.PersistentFlags().Int("memory", 2048, "memory in MB")
	FlagVmStartCores = vmStartCommand.PersistentFlags().Int("cores", 2, "number of CPU cores")
}

func command_vm_start(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	pac := getPACFromFlags()

	cluster, err := pac.Cluster(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster gave err: %w", err)
	}

	vmid, err := cluster.NextID(ctx)
	if err != nil {
		return fmt.Errorf("getting next VM ID gave err: %w", err)
	}

	node, err := pac.Node(ctx, *FlagVmStartNode)
	if err != nil {
		return fmt.Errorf("getting node %s gave err: %w", *FlagVmStartNode, err)
	}

	vmName := fmt.Sprintf("dtt-vm-%d", vmid)
	if *FlagVmStartName != "" {
		vmName = *FlagVmStartName
	}
	opts := []proxmox.VirtualMachineOption{
		{Name: "name", Value: vmName},
		{Name: "memory", Value: *FlagVmStartMemory},
		{Name: "cores", Value: *FlagVmStartCores},
		{Name: "sockets", Value: 1},
		{Name: "scsihw", Value: "virtio-scsi-pci"},
		{Name: "net0", Value: "virtio,bridge=vmbr0"},
	}

	task, err := node.NewVirtualMachine(ctx, vmid, opts...)
	if err != nil {
		return fmt.Errorf("creating VM %d gave err: %w", vmid, err)
	}

	if err := task.Wait(ctx, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for VM creation gave err: %w", err)
	}

	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		return fmt.Errorf("getting VM %d gave err: %w", vmid, err)
	}

	startTask, err := vm.Start(ctx)
	if err != nil {
		return fmt.Errorf("starting VM %d gave err: %w", vmid, err)
	}
	if err := startTask.Wait(ctx, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for VM start gave err: %w", err)
	}

	if err := vm.Ping(ctx); err != nil {
		return fmt.Errorf("pinging VM %d gave err: %w", vmid, err)
	}

	fmt.Printf("created and started vm %d (%s) on node %s\n", vmid, vmName, *FlagVmStartNode)

	return nil
}