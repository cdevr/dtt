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
	statusCommand = &cobra.Command{
		Use:   "status",
		Short: "Show the status of the Proxmox installation",
		RunE:  command_status,
	}
)

func init() {
	rootCmd.AddCommand(statusCommand)
}

func formatPercent(used uint64, total uint64) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", float64(used)*100.0/float64(total))
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}

	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}

	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	return fmt.Sprintf("%.1f %s", float64(value)/float64(div), suffixes[exp])
}

func formatUptime(seconds uint64) string {
	if seconds == 0 {
		return "0s"
	}

	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func command_status(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get Proxmox proxmox_client
	pac := getPACFromFlags()

	version, err := pac.Version(ctx)
	if err != nil {
		return fmt.Errorf("getting version gave err: %w", err)
	}
	fmt.Printf("Version: %s\n  version details: release %q version %q repoID %q\n\n", version.Version, version.Release, version.Version, version.RepoID)

	nodes, err := pac.Nodes(ctx)
	if err != nil {
		return fmt.Errorf("getting nodes gave err: %w", err)
	}

	fmt.Println("Nodes")
	nodeWriter := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(nodeWriter, "NODE\tSTATUS\tCPU\tMEM\tDISK\tUPTIME")
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Node < nodes[j].Node })
	for _, n := range nodes {
		fmt.Fprintf(
			nodeWriter,
			"%s\t%s\t%.1f%%\t%s/%s (%s)\t%s/%s (%s)\t%s\n",
			n.Node,
			n.Status,
			n.CPU*100.0,
			formatBytes(n.Mem),
			formatBytes(n.MaxMem),
			formatPercent(n.Mem, n.MaxMem),
			formatBytes(n.Disk),
			formatBytes(n.MaxDisk),
			formatPercent(n.Disk, n.MaxDisk),
			formatUptime(n.Uptime),
		)
	}
	if err := nodeWriter.Flush(); err != nil {
		return fmt.Errorf("flushing node writer gave err: %w", err)
	}

	cluster, err := pac.Cluster(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster gave err: %w", err)
	}

	resources, err := cluster.Resources(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster resources gave err: %w", err)
	}

	storageRows := make([]struct {
		Node   string
		Name   string
		Type   string
		Status string
		Used   uint64
		Total  uint64
	}, 0, len(resources))

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
		case "storage":
			storageRows = append(storageRows, struct {
				Node   string
				Name   string
				Type   string
				Status string
				Used   uint64
				Total  uint64
			}{
				Node:   r.Node,
				Name:   r.Storage,
				Type:   r.PluginType,
				Status: r.Status,
				Used:   r.Disk,
				Total:  r.MaxDisk,
			})
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

	sort.Slice(storageRows, func(i, j int) bool {
		if storageRows[i].Node == storageRows[j].Node {
			return storageRows[i].Name < storageRows[j].Name
		}
		return storageRows[i].Node < storageRows[j].Node
	})
	sort.Slice(vmRows, func(i, j int) bool {
		if vmRows[i].Node == vmRows[j].Node {
			return vmRows[i].VMID < vmRows[j].VMID
		}
		return vmRows[i].Node < vmRows[j].Node
	})

	fmt.Println()
	fmt.Println("Storage")
	storageWriter := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(storageWriter, "NODE\tSTORAGE\tTYPE\tSTATUS\tUSED\tTOTAL\tUSE%")
	for _, s := range storageRows {
		fmt.Fprintf(
			storageWriter,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.Node,
			s.Name,
			s.Type,
			s.Status,
			formatBytes(s.Used),
			formatBytes(s.Total),
			formatPercent(s.Used, s.Total),
		)
	}
	if err := storageWriter.Flush(); err != nil {
		return fmt.Errorf("flushing storage writer gave err: %w", err)
	}

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
		return fmt.Errorf("flushing vm writer gave err: %w", err)
	}

	return nil
}