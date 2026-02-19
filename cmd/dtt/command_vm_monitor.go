package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	vmMonitorCommand = &cobra.Command{
		Use:   "monitor <name-or-id>",
		Short: "monitor VM serial console output",
		Args:  cobra.ExactArgs(1),
		RunE:  command_vm_monitor,
	}

	FlagVmMonitorNode *string
)

func init() {
	FlagVmMonitorNode = vmMonitorCommand.PersistentFlags().String("node", "", "which node the VM is on")
	vmCommand.AddCommand(vmMonitorCommand)
}

func command_vm_monitor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	vm, err := findQemuVMForAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("finding VM gave err: %w", err)
	}

	term, err := vm.TermProxy(ctx)
	if err != nil {
		return fmt.Errorf("creating terminal proxy gave err: %w", err)
	}

	recvCh, sendCh, errCh, closeFn, err := vm.TermWebSocket(term)
	if err != nil {
		return fmt.Errorf("connecting to terminal websocket gave err: %w", err)
	}
	defer closeFn()

	// Handle Ctrl+C gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("Connected to VM %d serial console (Ctrl+C to exit)\n", vm.VMID)

	// Read from stdin and send to VM
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "stdin read error: %v\n", err)
				}
				return
			}
			if n > 0 {
				sendCh <- buf[:n]
			}
		}
	}()

	// Main loop
	for {
		select {
		case data := <-recvCh:
			os.Stdout.Write(data)
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("websocket error: %w", err)
			}
			return nil
		case <-sigCh:
			fmt.Println("\nDisconnecting...")
			return nil
		}
	}
}
