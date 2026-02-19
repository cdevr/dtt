package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	vmGetCommand = &cobra.Command{
		Use:   "get <name-or-id>",
		Short: "get vm details",
		Args:  cobra.ExactArgs(1),
		RunE:  command_vm_get,
	}
)

func init() {
	vmCommand.AddCommand(vmGetCommand)
}

func command_vm_get(cmd *cobra.Command, args []string) error {
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

	query := args[0]
	vmid, vmidQuery := parseVMIDArg(query)

	type vmResource struct {
		ID       string
		Node     string
		VMID     uint64
		Name     string
		Status   string
		CPU      float64
		Mem      uint64
		MaxMem   uint64
		Disk     uint64
		MaxDisk  uint64
		Uptime   uint64
		Template uint64
		Tags     string
		Pool     string
	}

	vmMatches := make([]vmResource, 0, 1)
	for _, r := range resources {
		if r.Type != "qemu" {
			continue
		}

		if vmidQuery {
			if r.VMID != vmid {
				continue
			}
		} else if r.Name != query {
			continue
		}

		vmMatches = append(vmMatches, vmResource{
			ID:       r.ID,
			Node:     r.Node,
			VMID:     r.VMID,
			Name:     r.Name,
			Status:   r.Status,
			CPU:      r.CPU,
			Mem:      r.Mem,
			MaxMem:   r.MaxMem,
			Disk:     r.Disk,
			MaxDisk:  r.MaxDisk,
			Uptime:   r.Uptime,
			Template: r.Template,
			Tags:     r.Tags,
			Pool:     r.Pool,
		})
	}

	if len(vmMatches) == 0 {
		return fmt.Errorf("vm %q not found", query)
	}

	if !vmidQuery && len(vmMatches) > 1 {
		return fmt.Errorf("multiple VMs found named %q; use vm id instead", query)
	}

	if !vmidQuery && len(vmMatches) > 1 {
		return fmt.Errorf("multiple VMs found named %q; use vm id instead", query)
	}

	vm := vmMatches[0]

	if len(vmMatches) > 1 {
		return fmt.Errorf("multiple VMs found named %q; use vm id instead", query)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "FIELD\tVALUE")
	fmt.Fprintf(writer, "id\t%s\n", vm.ID)
	fmt.Fprintf(writer, "node\t%s\n", vm.Node)
	fmt.Fprintf(writer, "vmid\t%d\n", vm.VMID)
	fmt.Fprintf(writer, "name\t%s\n", vm.Name)
	fmt.Fprintf(writer, "status\t%s\n", vm.Status)
	fmt.Fprintf(writer, "cpu\t%.1f%%\n", vm.CPU*100.0)
	fmt.Fprintf(writer, "memory\t%s / %s (%s)\n", formatBytes(vm.Mem), formatBytes(vm.MaxMem), formatPercent(vm.Mem, vm.MaxMem))
	fmt.Fprintf(writer, "disk\t%s / %s (%s)\n", formatBytes(vm.Disk), formatBytes(vm.MaxDisk), formatPercent(vm.Disk, vm.MaxDisk))
	fmt.Fprintf(writer, "uptime\t%s\n", formatUptime(vm.Uptime))
	fmt.Fprintf(writer, "template\t%t\n", vm.Template == 1)
	if vm.Pool != "" {
		fmt.Fprintf(writer, "pool\t%s\n", vm.Pool)
	}
	if vm.Tags != "" {
		fmt.Fprintf(writer, "tags\t%s\n", vm.Tags)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing vm details writer gave err: %w", err)
	}
	return nil
}

func parseVMIDArg(s string) (uint64, bool) {
	vmid, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return vmid, true
}