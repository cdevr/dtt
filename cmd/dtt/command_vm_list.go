package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	vmListCommand = &cobra.Command{
		Use:   "list",
		Short: "list vms",
		RunE:  command_vm_list,
	}
)

func init() {
	vmCommand.AddCommand(vmListCommand)
}

func command_vm_list(cmd *cobra.Command, args []string) error {
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

	vmRows := make([]struct {
		Node    string
		VMID    uint64
		Name    string
		Status  string
		CPU     float64
		Mem     uint64
		MaxMem  uint64
		Disk    uint64
		MaxDisk uint64
		Uptime  uint64
	}, 0, len(resources))

	for _, r := range resources {
		switch r.Type {
		case "qemu":
			vmRows = append(vmRows, struct {
				Node    string
				VMID    uint64
				Name    string
				Status  string
				CPU     float64
				Mem     uint64
				MaxMem  uint64
				Disk    uint64
				MaxDisk uint64
				Uptime  uint64
			}{
				Node:    r.Node,
				VMID:    r.VMID,
				Name:    r.Name,
				Status:  r.Status,
				CPU:     r.CPU,
				Mem:     r.Mem,
				MaxMem:  r.MaxMem,
				Disk:    r.Disk,
				MaxDisk: r.MaxDisk,
				Uptime:  r.Uptime,
			})
		}
	}

	sort.Slice(vmRows, func(i, j int) bool {
		if vmRows[i].Node == vmRows[j].Node {
			return vmRows[i].VMID < vmRows[j].VMID
		}
		return vmRows[i].Node < vmRows[j].Node
	})

	fmt.Println()
	fmt.Println("VMs")
	vmWriter := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(vmWriter, "NODE\tVMID\tNAME\tSTATUS\tCPU\tMEM\tDISK\tUPTIME")
	for _, vm := range vmRows {
		fmt.Fprintf(
			vmWriter,
			"%s\t%d\t%s\t%s\t%.1f%%\t%s/%s (%s)\t%s/%s (%s)\t%s\n",
			vm.Node,
			vm.VMID,
			vm.Name,
			vm.Status,
			vm.CPU*100.0,
			formatBytes(vm.Mem),
			formatBytes(vm.MaxMem),
			formatPercent(vm.Mem, vm.MaxMem),
			formatBytes(vm.Disk),
			formatBytes(vm.MaxDisk),
			formatPercent(vm.Disk, vm.MaxDisk),
			formatUptime(vm.Uptime),
		)
	}
	if err := vmWriter.Flush(); err != nil {
		return fmt.Errorf("flushing VM list writer gave err: %w", err)
	}

	return nil
}