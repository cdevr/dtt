package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	vmMonitorCommand = &cobra.Command{
		Use:   "monitor <name-or-id>",
		Short: "monitor VM serial console output",
		Args:  cobra.ExactArgs(1),
		RunE:  command_vm_monitor,
	}

	FlagVmMonitorNode  *string
	FlagVmMonitorQuiet *time.Duration
	FlagVmMonitorMax   *time.Duration
)

func init() {
	FlagVmMonitorNode = vmMonitorCommand.PersistentFlags().String("node", "", "which node the VM is on")
	FlagVmMonitorQuiet = vmMonitorCommand.PersistentFlags().Duration("quiet", 3*time.Second, "stop after no websocket output for this duration")
	FlagVmMonitorMax = vmMonitorCommand.PersistentFlags().Duration("max-duration", 30*time.Second, "maximum time to monitor websocket output")
	vmCommand.AddCommand(vmMonitorCommand)
}

func monitorVM(ctx context.Context, vm *proxmox.VirtualMachine, maxSilence, timeout time.Duration) ([]byte, error) {
	var result bytes.Buffer

	term, err := vm.TermProxy(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating terminal proxy gave err: %w", err)
	}
	fmt.Printf("got termproxy response: %v", term)
	fmt.Printf("Ticket is %s", term.Ticket)

	wsConn, err := vm.TermWebSocketConn(term)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket serial console monitor: %w", err)
	}
	defer wsConn.Close()

	totalDeadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(totalDeadline)
		if remaining <= 0 {
			break
		}

		readWait := maxSilence
		if readWait <= 0 || readWait > remaining {
			readWait = remaining
		}

		if err := wsConn.SetReadDeadline(time.Now().Add(readWait)); err != nil {
			return nil, fmt.Errorf("failed to set websocket read deadline: %w", err)
		}

		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				break
			}
			return nil, fmt.Errorf("error from websocket: %w", err)
		}

		result.Write(msg)
	}

	return result.Bytes(), nil
}

func command_vm_monitor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	query := args[0]
	vmid, vmidQuery := parseVMIDArg(query)

	pac := getPACFromFlags()

	cluster, err := pac.Cluster(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster gave err: %w", err)
	}

	resources, err := cluster.Resources(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster resources gave err: %w", err)
	}

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

	fVM := vmMatches[0]

	if len(vmMatches) > 1 {
		return fmt.Errorf("multiple VMs found named %q; use vm id instead", query)
	}

	node, err := pac.Node(ctx, fVM.Node)
	if err != nil {
		return fmt.Errorf("error getting node %q for VM %q (ID %s): %w", fVM.Node, fVM.Name, fVM.ID, err)
	}

	vm, err := node.VirtualMachine(ctx, int(fVM.VMID))
	if err != nil {
		return fmt.Errorf("getting VM gave err: %w", err)
	}

	_ = vm

	return nil
}
