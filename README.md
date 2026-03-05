# DTT - Do The Thing

A powerful CLI tool for running Linux binaries on Proxmox VMs with ease.

## Features

- **One-Command Workflows**: Spin up a VM, run your binary, and clean up in a single command
- **Automatic SSH Key Generation**: Ephemeral Ed25519 keys generated per-session for secure access
- **Binary Execution**: Upload and run Linux binaries on Proxmox VMs via SCP/SSH
- **Image Management**: Automatic image download and caching (Debian 10-13, Ubuntu 16.04-24.04)
- **Cloud-Init Support**: Automatic VM configuration via cloud-init
- **Live Boot Output**: Stream VM console output in real-time with `--verbose-boot`
- **Ephemeral VMs**: Auto-delete VMs after execution with `--delete`
- **VM Management**: Full lifecycle management (create, start, stop, delete, monitor)
- **Library API**: Use DTT as a library in your own Go programs
- **Bash/Zsh Completion**: Full shell completion support

## Installation

```bash
cd /Users/cde/src/dtt
go install ./cmd/dtt
```

## Quick Start

### Create a VM and run a binary (fully automated)

```bash
# Spin up a VM, upload a binary, execute it, and delete the VM when done
dtt vm cloudinit --binary ./my-program --delete \
  --proxmox-host proxmox.example.com \
  --proxmox-user root@pam
```

This single command:
1. Generates an ephemeral SSH key pair
2. Creates an Ubuntu VM with cloud-init
3. Waits for the VM to boot and get an IP
4. Uploads your binary via SCP
5. Executes the binary and prints output
6. Deletes the VM when done

### Create a VM with verbose boot output

```bash
dtt vm cloudinit --verbose-boot --binary ./my-program \
  --proxmox-host proxmox.example.com
```

### Use your own SSH key

```bash
dtt vm cloudinit \
  --sshkey "$(cat ~/.ssh/id_rsa.pub)" \
  --ssh-private-key ~/.ssh/id_rsa \
  --binary ./my-program
```

### List available images

```bash
dtt image list
```

### Download an image for faster provisioning

```bash
dtt image download "Debian 11" --storage local
```

### Manage VMs

```bash
# List all VMs
dtt vm list

# Delete a VM
dtt vm delete 100

# Monitor VM console output
dtt vm monitor 100
```

## Configuration

### Environment Variables

- `DTT_PROXMOX_PASSWORD`: Proxmox API password (avoid passing on command line)

### Global Flags

All commands support these Proxmox connection flags:

- `--proxmox-host`: Proxmox server hostname (default: localhost)
- `--proxmox-port`: Proxmox API port (default: 8006)
- `--proxmox-user`: API username (default: root@pam)
- `--proxmox-node`: Node name (default: pve)
- `--proxmox-insecure`: Skip SSL verification (default: false)

## Command Reference

### dtt run

Upload and execute a binary on a Proxmox VM.

**Usage**: `dtt run <binary-path> <vm-id> [flags]`

**Flags**:
- `--hostname`: VM hostname (default: dtt-vm)
- `--image`: Image to use: debian-11, debian-13, ubuntu-24.04 (default: debian-11)
- `--memory`: Memory in MB (default: 512)
- `--cpu`: Number of CPUs (default: 1)
- `--cores`: Cores per CPU (default: 1)
- `--username`: Default user (default: dtt)
- `--remote-path`: Path to place binary on VM (default: /tmp/binary)

### dtt image

Manage VM images.

**Subcommands**:
- `list`: List available images
- `download`: Download an image to Proxmox storage

### dtt vm

Manage virtual machines.

**Subcommands**:
- `list`: List all VMs on the node
- `delete`: Delete a VM
- `cloudinit`: Create a cloud-init VM and optionally run a binary
- `start`, `stop`, `restart`, `shutdown`, `reset`: VM power management
- `monitor`: Stream VM console output
- `get`: Get VM details

### dtt vm cloudinit

Create a VM from a cloud image with cloud-init configuration, optionally upload and execute a binary.

**Usage**: `dtt vm cloudinit [flags]`

**Flags**:
- `--node`: Proxmox node name (default: pve)
- `--name`: VM name (default: auto-generated)
- `--release`: OS release, e.g., ubuntu:noble, debian:bookworm (default: ubuntu:noble)
- `--memory`: Memory in MB (default: 2048)
- `--cores`: CPU cores (default: 2)
- `--disk-size`: Additional disk size (default: +10G)
- `--storage`: Storage location (default: local)
- `--username`: Cloud-init username (default: dtt)
- `--password`: Cloud-init password (auto-generated if not set)
- `--sshkey`: SSH public key or "generate" for auto-generation (default: generate)
- `--ssh-private-key`: Path to SSH private key for connecting
- `--binary`: Local binary/script to upload and execute
- `--remote-path`: Remote path for binary (default: /tmp)
- `--args`: Arguments to pass to the binary
- `--verbose-boot`: Print VM boot console output in real-time
- `--delete`: Delete the VM after completion (success or failure)
- `--net`: Network device options (can specify multiple)
- `--pool`: Resource pool for the VM

**Examples**:
```bash
# Minimal: create VM with auto-generated SSH key
dtt vm cloudinit

# Run a binary and clean up
dtt vm cloudinit --binary ./test-program --delete

# Watch boot process and run a script
dtt vm cloudinit --verbose-boot --binary ./setup.sh --args "--config prod"

# Use Debian instead of Ubuntu
dtt vm cloudinit --release debian:bookworm --binary ./my-app
```

### dtt completion

Generate shell completion scripts.

**Usage**: `dtt completion [bash|zsh|fish|powershell]`

**Examples**:
```bash
# Bash
source <(dtt completion bash)

# Zsh
source <(dtt completion zsh)
```

## Using DTT as a Library

DTT is organized as a Go module with separate packages for different functionality:

### Proxmox Client

```go
import "github.com/cdevr/dtt/pkg/proxmox"

client := proxmox.NewClient(proxmox.ClientConfig{
    Host:     "proxmox.example.com",
    Port:     8006,
    Username: "root@pam",
    Password: "password",
    Node:     "pve",
})

vmSpec := proxmox.VMSpec{
    Name:   "my-vm",
    VMID:   100,
    Image:  proxmox.DefaultImages()[0],
    Memory: 2048,
    CPU:    2,
}

vm, err := client.CreateVM(vmSpec)
```

### Cloud-Init Configuration

```go
import "github.com/cdevr/dtt/pkg/cloudconfig"

config := cloudconfig.NewBuilder().
    WithHostname("my-vm").
    WithUsername("ubuntu").
    WithPackage("curl").
    WithPackage("git").
    WithRunCommand("apt-get update").
    Build()

userDataYAML := config.Generate()
```

### Binary Management

```go
import "github.com/cdevr/dtt/pkg/binary"

// Get binary information
info, err := binary.GetBinaryInfo("/path/to/binary")

// Validate binary
err := binary.ValidateBinary("/path/to/binary")

// Verify binary hash
err := binary.VerifyBinary("/path/to/binary", expectedMD5, expectedSHA256)
```

## Project Structure

```
dtt/
├── cmd/dtt/              # CLI application
│   ├── main.go
│   └── commands/         # Command implementations
│       ├── root.go
│       ├── commands.go
│       └── commands_test.go
├── pkg/                  # Reusable packages
│   ├── proxmox/         # Proxmox API client
│   │   ├── client.go
│   │   └── client_test.go
│   ├── cloudconfig/     # Cloud-init configuration
│   │   ├── cloudconfig.go
│   │   └── cloudconfig_test.go
│   └── binary/          # Binary management
│       ├── binary.go
│       └── binary_test.go
├── internal/            # Internal packages
│   └── config/          # Configuration management
├── go.mod
├── go.sum
└── README.md
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/proxmox/...
go test ./pkg/cloudconfig/...
go test ./pkg/binary/...
```

## Development

### Code Style

This project follows standard Go conventions:
- `gofmt` for formatting
- `golint` for linting
- Clear package organization
- Comprehensive comments for public APIs

### Adding New Features

1. Create new packages in `pkg/` for reusable functionality
2. Add CLI commands in `cmd/dtt/commands/`
3. Write tests for all new packages
4. Update documentation

## Architecture

DTT is designed with separation of concerns:

- **pkg/proxmox**: Low-level Proxmox API interactions
- **pkg/cloudconfig**: Cloud-init configuration generation
- **pkg/binary**: Binary validation and management
- **cmd/dtt**: High-level CLI orchestration

This allows the core functionality to be used as a library independent of the CLI.

## License

MIT

## Support

For issues, questions, or contributions, please refer to the project repository.
