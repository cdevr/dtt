package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cdevr/dtt/parseCloudInitLog"
	"github.com/luthermonson/go-proxmox"
	"github.com/spf13/cobra"
)

var (
	vmCloudInitCommand = &cobra.Command{
		Use:   "cloudinit",
		Short: "create a VM from Ubuntu minimal cloud image with cloud-init and start it",
		RunE:  command_vm_cloudinit,
	}

	FlagVmCloudInitNode           *string
	FlagVmCloudInitName           *string
	FlagVmCloudInitMemory         *int
	FlagVmCloudInitCores          *int
	FlagVmCloudInitStorage        *string
	FlagVmCloudInitRelease        *string
	FlagVmCloudInitDiskSize       *string
	FlagVmCloudInitUsername       *string
	FlagVmCloudInitPassword       *string
	FlagVmCloudInitSSHKey         *string
	FlagVmCloudInitPool           *string
	FlagVmCloudInitNetworkDevice  *[]string
	FlagVmCloudInitLogMonitorFile *string
)

func init() {
	vmCommand.AddCommand(vmCloudInitCommand)

	FlagVmCloudInitNode = vmCloudInitCommand.PersistentFlags().String("node", "pve", "which node to create the vm on")
	FlagVmCloudInitName = vmCloudInitCommand.PersistentFlags().String("name", "", "name of vm to create (default: dtt-ubuntu-<release>-<id>)")
	FlagVmCloudInitMemory = vmCloudInitCommand.PersistentFlags().Int("memory", 2048, "memory in MB")
	FlagVmCloudInitCores = vmCloudInitCommand.PersistentFlags().Int("cores", 2, "number of CPU cores")
	FlagVmCloudInitStorage = vmCloudInitCommand.PersistentFlags().String("storage", "local", "storage for imported disk and cloud-init drive")
	FlagVmCloudInitRelease = vmCloudInitCommand.PersistentFlags().String("release", "ubuntu:noble", "the version you want, default is ubuntu:noble (can be bionic, focal, jammy, noble, plucky, questing, xenial, 22.04, 20.04), can also be debian:bullseye (can be buster, bullseye, bookworm, trixie, 11, 13)")
	FlagVmCloudInitDiskSize = vmCloudInitCommand.PersistentFlags().String("disk-size", "+10G", "additional size for boot disk resize (e.g. +10G)")
	FlagVmCloudInitUsername = vmCloudInitCommand.PersistentFlags().String("username", "dtt", "cloud-init username")
	FlagVmCloudInitPassword = vmCloudInitCommand.PersistentFlags().String("password", "", "cloud-init password")
	FlagVmCloudInitSSHKey = vmCloudInitCommand.PersistentFlags().String("sshkey", "", "cloud-init SSH public key")
	FlagVmCloudInitPool = vmCloudInitCommand.PersistentFlags().String("pool", "", "resource pool to create the node in")
	FlagVmCloudInitNetworkDevice = vmCloudInitCommand.PersistentFlags().StringArray("net", []string{"virtio,bridge=vmbr0"}, "network device options, for example you can add tag= for a VLAN tag. You can add none of these, or many")
	FlagVmCloudInitLogMonitorFile = vmCloudInitCommand.PersistentFlags().String("monitorfile", "", "log VM monitor data to file")
}

var (
	distro_versions = map[string]map[string]string{
		"debian": map[string]string{
			"buster":   "10",
			"bullseye": "11",
			"bookworm": "12",
			"trixie":   "13",
		}, "ubuntu": map[string]string{
			"xenial": "16.04",
			"bionic": "18.04",
			"focal":  "20.04",
			"jammy":  "22.04",
			"noble":  "24.04",
		},
	}
)

func command_vm_cloudinit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	pac := getPACFromFlags()

	cluster, err := pac.Cluster(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster gave err: %w", err)
	}

	vmID, err := cluster.NextID(ctx)
	if err != nil {
		return fmt.Errorf("getting next VM ID gave err: %w", err)
	}

	node, err := pac.Node(ctx, *FlagVmCloudInitNode)
	if err != nil {
		return fmt.Errorf("getting node %s gave err: %w", *FlagVmCloudInitNode, err)
	}

	release := strings.TrimSpace(*FlagVmCloudInitRelease)
	if release == "" {
		return fmt.Errorf("release cannot be empty")
	}

	distro, version, err := extractDistroVersionFromRelease(release)
	if err != nil {
		return err
	}

	cloudImageURL, err := getFnFromCloudImageURL(distro, version, release)
	if err != nil {
		return fmt.Errorf("Failed to get cloudImageURL: %w", err)
	}
	log.Printf("constructed cloudImageURL: %q", cloudImageURL)

	qcow2Name, err := extractFn(cloudImageURL)
	if err != nil {
		return fmt.Errorf("failed to extract filename from URL %q", cloudImageURL)
	}

	// Needed for ubuntu minimal cloud images.
	qcow2Name = strings.ReplaceAll(qcow2Name, ".img", ".qcow2")
	importVolID := fmt.Sprintf("%s:import/%s", *FlagVmCloudInitStorage, qcow2Name)

	storage, err := node.Storage(ctx, *FlagVmCloudInitStorage)
	if err != nil {
		return fmt.Errorf("getting storage %s on node %s gave err: %w", *FlagVmCloudInitStorage, *FlagVmCloudInitNode, err)
	}

	if err := ensureImportImage(ctx, storage, qcow2Name, cloudImageURL); err != nil {
		return fmt.Errorf("importing cloud image gave err: %w", err)
	}

	vmName := fmt.Sprintf("dtt-%s-%d", strings.Replace(release, ":", "-", -1), vmID)
	if *FlagVmCloudInitName != "" {
		vmName = *FlagVmCloudInitName
	}

	opts := []proxmox.VirtualMachineOption{
		proxmox.VirtualMachineOption{Name: "name", Value: vmName},
		proxmox.VirtualMachineOption{Name: "memory", Value: *FlagVmCloudInitMemory},
		proxmox.VirtualMachineOption{Name: "cores", Value: *FlagVmCloudInitCores},
		proxmox.VirtualMachineOption{Name: "sockets", Value: 1},
		proxmox.VirtualMachineOption{Name: "ostype", Value: "l26"},
		proxmox.VirtualMachineOption{Name: "scsihw", Value: "virtio-scsi-pci"},
		proxmox.VirtualMachineOption{Name: "serial0", Value: "socket"},
		proxmox.VirtualMachineOption{Name: "vga", Value: "serial0"},
		proxmox.VirtualMachineOption{Name: "agent", Value: "enabled=1"},
	}
	for i, netdev := range *FlagVmCloudInitNetworkDevice {
		opts = append(opts, proxmox.VirtualMachineOption{Name: fmt.Sprintf("net%d", i), Value: netdev})
	}
	if *FlagVmCloudInitPool != "" {
		opts = append(opts, proxmox.VirtualMachineOption{"pool", *FlagVmCloudInitPool})
	}
	log.Printf("creating VM with ID %d and params: %v", vmID, opts)

	createTask, err := node.NewVirtualMachine(
		ctx,
		vmID,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("creating cloud-init VM %d gave err: %w", vmID, err)
	}
	if err := createTask.Wait(ctx, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for cloud-init VM creation gave err: %w", err)
	}

	vm, err := node.VirtualMachine(ctx, vmID)
	if err != nil {
		return fmt.Errorf("getting cloud-init VM %d gave err: %w", vmID, err)
	}

	ciPassword := *FlagVmCloudInitPassword
	if strings.TrimSpace(ciPassword) == "" {
		ciPassword, err = GenerateEasyPassword(3)
		if err != nil {
			return fmt.Errorf("failed to generate easy password: %w", err)
		}
		fmt.Printf("generated cloud-init credentials: username %s password %s\n", *FlagVmCloudInitUsername, ciPassword)
	}

	log.Printf("configuring VM %q ID %d with boot drive, and cloud init parameters", vm.Name, vm.VMID)
	configOpts := []proxmox.VirtualMachineOption{
		proxmox.VirtualMachineOption{Name: "scsi0", Value: fmt.Sprintf("%s:0,import-from=%s", *FlagVmCloudInitStorage, importVolID)},
		proxmox.VirtualMachineOption{Name: "boot", Value: "order=scsi0"},
		proxmox.VirtualMachineOption{Name: "ide2", Value: fmt.Sprintf("%s:cloudinit", *FlagVmCloudInitStorage)},
		proxmox.VirtualMachineOption{Name: "ciuser", Value: *FlagVmCloudInitUsername},
		proxmox.VirtualMachineOption{Name: "cipassword", Value: ciPassword},
		proxmox.VirtualMachineOption{Name: "ipconfig0", Value: "ip=dhcp,ip6=auto"},
	}
	if sshKey := strings.TrimSpace(*FlagVmCloudInitSSHKey); sshKey != "" {
		enc := url.QueryEscape(sshKey)            // makes spaces into +
		enc = strings.ReplaceAll(enc, "+", "%20") // turn the + encoded spaces into %20

		log.Printf("passing in sshkeys %q", enc)

		configOpts = append(configOpts, proxmox.VirtualMachineOption{Name: "sshkeys", Value: enc})
	}
	configTask, err := vm.Config(ctx, configOpts...)
	if err != nil {
		return fmt.Errorf("configuring cloud-init VM gave err: %w", err)
	}
	if err := configTask.Wait(ctx, time.Second, 5*time.Minute); err != nil {
		return fmt.Errorf("waiting for cloud-init config gave err: %w", err)
	}

	resizeTask, err := vm.ResizeDisk(ctx, "scsi0", *FlagVmCloudInitDiskSize)
	if err != nil {
		return fmt.Errorf("resizing cloud-init VM disk gave err: %w", err)
	}
	if err := resizeTask.Wait(ctx, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for disk resize gave err: %w", err)
	}

	startTask, err := vm.Start(ctx)
	if err != nil {
		return fmt.Errorf("starting cloud-init VM gave err: %w", err)
	}
	if err := startTask.Wait(ctx, time.Second, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for cloud-init VM start gave err: %w", err)
	}

	output, err := monitorVM(ctx, vm, 3*time.Second, 1*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to get cloudinit output for VM")
	}
	if *FlagVmCloudInitLogMonitorFile != "" {
		if err := os.WriteFile(*FlagVmCloudInitLogMonitorFile, []byte(output), 0o644); err != nil {
			return fmt.Errorf("failed to write monitor output to %q: %w", *FlagVmCloudInitLogMonitorFile, err)
		}
	}

	parsedOutput := parseCloudInitLog.ParseCloudInit(output)
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FIELD\tVALUE")
	fmt.Fprintln(tw, "-----\t-----")
	fmt.Fprintf(tw, "Hostname\t%s\n", parsedOutput.Hostname)
	if len(parsedOutput.IPs) == 0 {
		fmt.Fprintln(tw, "IPs\t(none)")
	} else {
		fmt.Fprintf(tw, "IPs\t%s\n", strings.Join(parsedOutput.IPs, ", "))
	}
	fmt.Fprintf(tw, "Host Key Hashes\t%d\n", len(parsedOutput.HostKeyHashes))
	for i, hk := range parsedOutput.HostKeyHashes {
		fmt.Fprintf(
			tw,
			"  [%d] %s\t%s (%s, %s)\n",
			i+1,
			hk.KeyType,
			hk.Fingerprint,
			hk.Algorithm,
			hk.Hostname,
		)
	}
	fmt.Fprintf(tw, "Host Keys\t%d\n", len(parsedOutput.HostKeys))
	for i, key := range parsedOutput.HostKeys {
		fmt.Fprintf(tw, "  [%d]\t%s\n", i+1, key)
	}
	fmt.Fprintf(tw, "Authorized SSH Keys\t%d\n", len(parsedOutput.SSHKeyData))
	if len(parsedOutput.SSHKeyData) == 0 {
		fmt.Fprintln(tw, "  Users\t(none)")
	} else {
		for user, keyData := range parsedOutput.SSHKeyData {
			fmt.Fprintf(tw, "  User\t%s\n", user)
			fmt.Fprintf(tw, "    Key Type\t%s\n", keyData.Keytype)
			fmt.Fprintf(tw, "    Fingerprint\t%s\n", keyData.FingerPrint)
			if keyData.Options == "" {
				fmt.Fprintln(tw, "    Options\t(none)")
			} else {
				fmt.Fprintf(tw, "    Options\t%s\n", keyData.Options)
			}
			if keyData.Comment == "" {
				fmt.Fprintln(tw, "    Comment\t(none)")
			} else {
				fmt.Fprintf(tw, "    Comment\t%s\n", keyData.Comment)
			}
		}
	}
	_ = tw.Flush()

	fmt.Printf("created and started cloud-init vm %d (%s) on node %s from %s\n", vmID, vmName, *FlagVmCloudInitNode, cloudImageURL)
	return nil
}

func extractDistroVersionFromRelease(release string) (string, string, error) {
	distro := ""
	version := ""
	if strings.Contains(release, ":") {
		parts := strings.SplitN(release, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("this should not happen: %q split into %v", release, parts)
		}
		distro = parts[0]
		version = parts[1]
		log.Printf("distro: %q version: %q", distro, version)

		// Allow identifying distros by version, e.g. "debian:11"
		if distro, distroFound := distro_versions[distro]; !distroFound {
			return "", "", fmt.Errorf("distro %q not found in list", distro)
		} else {
			for name, ver := range distro {
				if version == ver {
					version = name
				}
			}
		}
		log.Printf("distro: %q version: %q", distro, version)
	}
	return distro, version, nil
}

func GetIPFor(ctx context.Context, vm *proxmox.VirtualMachine, attempts int, delay time.Duration) (string, error) {
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		interfaces, err := vm.AgentGetNetworkIFaces(ctx)
		if err == nil {
			for _, iface := range interfaces {
				for _, addr := range iface.IPAddresses {
					ip := net.ParseIP(addr.IPAddress)
					if ip == nil {
						continue
					}

					// Skip loopback + non-IPv4
					if ip.IsLoopback() || ip.To4() == nil {
						continue
					}

					return ip.String(), nil
				}
			}
		}

		time.Sleep(delay)
	}

	return "", errors.New("timeout waiting for VM IP address")
}

func getFnFromCloudImageURL(distro string, version string, release string) (string, error) {
	switch distro {
	case "ubuntu":
		return fmt.Sprintf(
			"https://cloud-images.ubuntu.com/minimal/daily/%s/current/%s-minimal-cloudimg-amd64.img",
			version,
			version,
		), nil
	case "debian":
		debRelease, ok := distro_versions["debian"][version]
		if !ok {
			return "", fmt.Errorf("unknown debian release %q in release specifier %q", version, release)
		}
		return fmt.Sprintf(
			"https://cdimage.debian.org/images/cloud/%s/latest/debian-%s-generic-amd64.qcow2",
			version,
			debRelease,
		), nil
	default:
		return "", fmt.Errorf("can't recognize distro (ubuntu or debian) in %q from %q", distro, release)
	}
}

func ensureImportImage(ctx context.Context, storage *proxmox.Storage, filename, imageURL string) error {
	content, err := storage.GetContent(ctx)
	if err != nil {
		return fmt.Errorf("getting storage content gave err: %w", err)
	}
	for _, c := range content {
		if c.Volid == fmt.Sprintf("%s:import/%s", storage.Name, filename) {
			return nil
		}
	}

	task, err := storage.DownloadURL(ctx, "import", filename, imageURL)
	if err != nil {
		return fmt.Errorf("downloading image %s gave err: %w", imageURL, err)
	}
	if err := task.Wait(ctx, time.Second, 30*time.Minute); err != nil {
		return fmt.Errorf("waiting for image download gave err: %w", err)
	}
	return nil
}

// Generates a human-friendly password like:
// Vako7-Nemir3-Talop8
// still comes with 50 bits of entropy!
func GenerateEasyPassword(groups int) (string, error) {
	consonants := "bcdfghjkmnpqrstvwxyz"
	vowels := "aeiou"
	digits := "23456789" // removed 0 and 1

	var passwordParts []string

	for i := 0; i < groups; i++ {
		part, err := generateWord(consonants, vowels, digits)
		if err != nil {
			return "", err
		}
		passwordParts = append(passwordParts, part)
	}

	return strings.Join(passwordParts, "-"), nil
}

func generateWord(consonants, vowels, digits string) (string, error) {
	pattern := []string{consonants, vowels, consonants, vowels, consonants, digits}
	var result strings.Builder

	for _, charset := range pattern {
		ch, err := randomChar(charset)
		if err != nil {
			return "", err
		}
		result.WriteByte(ch)
	}

	word := result.String()
	return strings.Title(word), nil // Capitalize first letter
}

func randomChar(charset string) (byte, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return 0, err
	}
	return charset[nBig.Int64()], nil
}

func extractFn(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return path.Base(parsed.Path), nil
}
