package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	px "github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	agentListCommand = &cobra.Command{
		Use:   "list",
		Short: "list available qemu guest agent commands",
		RunE:  command_agent_list,
	}

	agentOsInfoCommand = &cobra.Command{
		Use:   "osinfo <name-or-id>",
		Short: "show guest OS info from qemu guest agent",
		Args:  cobra.ExactArgs(1),
		RunE:  command_agent_osinfo,
	}

	agentNetworkCommand = &cobra.Command{
		Use:   "network <name-or-id>",
		Short: "show guest network interfaces from qemu guest agent",
		Args:  cobra.ExactArgs(1),
		RunE:  command_agent_network,
	}

	agentExecCommand = &cobra.Command{
		Use:   "exec <name-or-id> <command> [args...]",
		Short: "execute a command in guest using qemu guest agent",
		Args:  cobra.MinimumNArgs(2),
		RunE:  command_agent_exec,
	}

	agentExecStatusCommand = &cobra.Command{
		Use:   "exec-status <name-or-id> <pid>",
		Short: "get status for a qemu guest agent exec pid",
		Args:  cobra.ExactArgs(2),
		RunE:  command_agent_exec_status,
	}

	agentSetUserPasswordCommand = &cobra.Command{
		Use:   "set-user-password <name-or-id>",
		Short: "set a guest user password using qemu guest agent",
		Args:  cobra.ExactArgs(1),
		RunE:  command_agent_set_user_password,
	}

	FlagAgentNode *string

	FlagAgentExecInput   *string
	FlagAgentExecWait    *bool
	FlagAgentExecTimeout *int

	FlagAgentSetUserPasswordUsername *string
	FlagAgentSetUserPasswordPassword *string
)

func init() {
	agentCommand.AddCommand(agentListCommand)
	agentCommand.AddCommand(agentOsInfoCommand)
	agentCommand.AddCommand(agentNetworkCommand)
	agentCommand.AddCommand(agentExecCommand)
	agentCommand.AddCommand(agentExecStatusCommand)
	agentCommand.AddCommand(agentSetUserPasswordCommand)

	FlagAgentNode = agentCommand.PersistentFlags().String("node", "", "limit VM lookup to a specific node")

	FlagAgentExecInput = agentExecCommand.Flags().String("input", "", "stdin input passed to agent exec")
	FlagAgentExecWait = agentExecCommand.Flags().Bool("wait", true, "wait for command completion")
	FlagAgentExecTimeout = agentExecCommand.Flags().Int("timeout", 30, "seconds to wait when --wait is true")

	FlagAgentSetUserPasswordUsername = agentSetUserPasswordCommand.Flags().String("username", "", "guest username")
	FlagAgentSetUserPasswordPassword = agentSetUserPasswordCommand.Flags().String("password", "", "new guest password")
	_ = agentSetUserPasswordCommand.MarkFlagRequired("username")
	_ = agentSetUserPasswordCommand.MarkFlagRequired("password")
}

func command_agent_list(cmd *cobra.Command, args []string) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "COMMAND\tDESCRIPTION")
	fmt.Fprintln(writer, "osinfo\tShow guest OS metadata")
	fmt.Fprintln(writer, "network\tShow guest network interfaces and IPs")
	fmt.Fprintln(writer, "exec\tExecute command in guest")
	fmt.Fprintln(writer, "exec-status\tGet status/output for exec pid")
	fmt.Fprintln(writer, "set-user-password\tUpdate guest user password")
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing agent list writer gave err: %w", err)
	}
	return nil
}

func command_agent_osinfo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	vm, err := findQemuVMForAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("finding VM for agent gave err: %w", err)
	}

	info, err := vm.AgentOsInfo(ctx)
	if err != nil {
		return fmt.Errorf("getting agent OS info gave err: %w", err)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "FIELD\tVALUE")
	fmt.Fprintf(writer, "name\t%s\n", info.Name)
	fmt.Fprintf(writer, "pretty_name\t%s\n", info.PrettyName)
	fmt.Fprintf(writer, "id\t%s\n", info.ID)
	fmt.Fprintf(writer, "version\t%s\n", info.Version)
	fmt.Fprintf(writer, "version_id\t%s\n", info.VersionID)
	fmt.Fprintf(writer, "machine\t%s\n", info.Machine)
	fmt.Fprintf(writer, "kernel_release\t%s\n", info.KernelRelease)
	fmt.Fprintf(writer, "kernel_version\t%s\n", info.KernelVersion)
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing agent osinfo writer gave err: %w", err)
	}
	return nil
}

func command_agent_network(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	vm, err := findQemuVMForAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("finding VM for agent network gave err: %w", err)
	}

	ifaces, err := vm.AgentGetNetworkIFaces(ctx)
	if err != nil {
		return fmt.Errorf("getting agent network interfaces gave err: %w", err)
	}

	sort.Slice(ifaces, func(i, j int) bool {
		return ifaces[i].Name < ifaces[j].Name
	})

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "INTERFACE\tMAC\tTYPE\tADDRESS")
	for _, iface := range ifaces {
		if len(iface.IPAddresses) == 0 {
			fmt.Fprintf(writer, "%s\t%s\t\t\n", iface.Name, iface.HardwareAddress)
			continue
		}
		for _, ip := range iface.IPAddresses {
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s/%d\n", iface.Name, iface.HardwareAddress, ip.IPAddressType, ip.IPAddress, ip.Prefix)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing agent network writer gave err: %w", err)
	}
	return nil
}

func command_agent_exec(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	vm, err := findQemuVMForAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("finding VM for agent exec gave err: %w", err)
	}

	guestCmd := args[1:]
	pid, err := vm.AgentExec(ctx, guestCmd, *FlagAgentExecInput)
	if err != nil {
		return fmt.Errorf("executing agent command gave err: %w", err)
	}

	if !*FlagAgentExecWait {
		fmt.Printf("pid: %d\n", pid)
		return nil
	}

	status, err := vm.WaitForAgentExecExit(ctx, pid, *FlagAgentExecTimeout)
	if err != nil {
		return fmt.Errorf("waiting for agent exec gave err: %w", err)
	}

	writeAgentExecOutputs(status)

	if status.ExitCode != 0 {
		return fmt.Errorf("agent exec failed: pid %d exit code %d", pid, status.ExitCode)
	}

	return nil
}

func command_agent_exec_status(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	vm, err := findQemuVMForAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("finding VM for agent exec-status gave err: %w", err)
	}

	pid, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid pid %q: %w", args[1], err)
	}

	status, err := vm.AgentExecStatus(ctx, pid)
	if err != nil {
		return fmt.Errorf("getting agent exec status gave err: %w", err)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "FIELD\tVALUE")
	fmt.Fprintf(writer, "pid\t%d\n", pid)
	fmt.Fprintf(writer, "exited\t%t\n", status.Exited != 0)
	fmt.Fprintf(writer, "exit_code\t%d\n", status.ExitCode)
	fmt.Fprintf(writer, "signal\t%t\n", status.Signal)
	fmt.Fprintf(writer, "out_truncated\t%s\n", status.OutTruncated)
	fmt.Fprintf(writer, "err_truncated\t%t\n", status.ErrTruncated)
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing agent exec-status writer gave err: %w", err)
	}

	writeAgentExecOutputs(status)
	return nil
}

func command_agent_set_user_password(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	vm, err := findQemuVMForAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("finding VM for set-user-password gave err: %w", err)
	}

	if err := vm.AgentSetUserPassword(ctx, *FlagAgentSetUserPasswordPassword, *FlagAgentSetUserPasswordUsername); err != nil {
		return fmt.Errorf("setting user password gave err: %w", err)
	}

	fmt.Printf("password updated for user %q\n", *FlagAgentSetUserPasswordUsername)
	return nil
}

func findQemuVMForAgent(ctx context.Context, query string) (*px.VirtualMachine, error) {
	pac := getPACFromFlags()
	cluster, err := pac.Cluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting cluster gave err: %w", err)
	}

	resources, err := cluster.Resources(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting cluster resources gave err: %w", err)
	}

	type candidate struct {
		Node string
		VMID uint64
		Name string
	}

	vmid, vmidQuery := parseVMIDArg(query)
	matches := make([]candidate, 0, 1)

	for _, r := range resources {
		if r.Type != "qemu" {
			continue
		}
		if strings.TrimSpace(*FlagAgentNode) != "" && r.Node != *FlagAgentNode {
			continue
		}

		if vmidQuery {
			if r.VMID != vmid {
				continue
			}
		} else if r.Name != query {
			continue
		}

		matches = append(matches, candidate{Node: r.Node, VMID: r.VMID, Name: r.Name})
	}

	if len(matches) == 0 {
		if strings.TrimSpace(*FlagAgentNode) != "" {
			return nil, fmt.Errorf("vm %q not found on node %q", query, *FlagAgentNode)
		}
		return nil, fmt.Errorf("vm %q not found", query)
	}

	if len(matches) > 1 {
		conflicts := make([]string, 0, len(matches))
		for _, m := range matches {
			conflicts = append(conflicts, fmt.Sprintf("%s/%d(%s)", m.Node, m.VMID, m.Name))
		}
		return nil, fmt.Errorf("multiple VMs matched %q: %s; pass VMID or --node", query, strings.Join(conflicts, ", "))
	}

	node, err := pac.Node(ctx, matches[0].Node)
	if err != nil {
		return nil, fmt.Errorf("getting node %s gave err: %w", matches[0].Node, err)
	}

	return node.VirtualMachine(ctx, int(matches[0].VMID))
}

func writeAgentExecOutputs(status *px.AgentExecStatus) {
	stdout := decodeAgentExecData(status.OutData)
	stderr := decodeAgentExecData(status.ErrData)

	if stdout != "" {
		_, _ = os.Stdout.WriteString(stdout)
		if !strings.HasSuffix(stdout, "\n") {
			_, _ = os.Stdout.WriteString("\n")
		}
	}
	if stderr != "" {
		_, _ = os.Stderr.WriteString(stderr)
		if !strings.HasSuffix(stderr, "\n") {
			_, _ = os.Stderr.WriteString("\n")
		}
	}
}

func decodeAgentExecData(s string) string {
	if s == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return string(decoded)
}