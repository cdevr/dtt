# CLAUDE.md — DTT (Do The Thing)

## Project Overview

DTT is a CLI tool and Go library for running Linux binaries on Proxmox VMs. It automates the full lifecycle: downloading cloud images, creating VMs with cloud-init configuration, uploading binaries via SSH, executing them, and cleaning up.

**Module**: `github.com/cdevr/dtt`
**Go Version**: 1.24.0
**CLI Framework**: [Cobra](https://github.com/spf13/cobra) v1.7.0

---

## Repository Structure

```
dtt/
├── cmd/dtt/                        # CLI entry point
│   ├── main.go                     # Cobra root command setup
│   ├── commands/                   # Subcommand package
│   │   ├── root.go                 # Global flags and root command
│   │   ├── commands.go             # Subcommand registration
│   │   ├── command_run.go          # 'run' — upload and execute a binary on a VM
│   │   └── commands_test.go        # Command unit tests
│   ├── command_vm_list.go          # vm list
│   ├── command_vm_start.go         # vm start
│   ├── command_vm_stop.go          # vm stop
│   ├── command_vm_restart.go       # vm restart
│   ├── command_vm_shutdown.go      # vm shutdown
│   ├── command_vm_reset.go         # vm reset
│   ├── command_vm_get.go           # vm get
│   ├── command_vm_delete.go        # vm delete
│   ├── command_vm_cloudinit.go     # vm cloudinit — show cloud-init output
│   ├── command_vm_monitor.go       # vm monitor — stream VM console output
│   ├── command_image_list.go       # image list
│   ├── command_image_upload.go     # image upload
│   ├── command_image_download.go   # image download
│   ├── command_image_rm.go         # image rm
│   ├── command_agent_list.go       # agent list
│   ├── command_agent_osinfo.go     # agent osinfo
│   ├── command_agent_network.go    # agent network
│   ├── command_agent_exec.go       # agent exec
│   ├── command_agent_execstatus.go # agent exec-status
│   └── command_status.go           # cluster status
├── pkg/                            # Reusable library packages
│   ├── proxmox/
│   │   ├── client.go               # Main Proxmox API client (854 lines)
│   │   └── client_test.go
│   ├── cloudconfig/
│   │   ├── cloudconfig.go          # Cloud-init YAML generation
│   │   └── cloudconfig_test.go
│   ├── binary/
│   │   ├── binary.go               # Binary metadata, hashing, validation
│   │   └── binary_test.go
│   └── ssh/
│       └── ssh.go                  # SSH client wrapper
├── parseCloudInitLog/
│   ├── parse.go                    # Regex-based cloud-init log parser
│   └── parse_test.go
├── examples/
│   ├── run_binary.sh
│   └── test_binary.sh
├── go.mod
├── go.sum
├── README.md
├── .env.example                    # Environment variable template
└── prompt                          # Original project specification
```

---

## Key Packages

### `pkg/proxmox` — Proxmox API Client

The main integration layer. All Proxmox operations go through `Client`.

```go
type ClientConfig struct {
    Host        string
    Port        int
    Username    string
    Password    string
    TokenID     string
    TokenSecret string
    Node        string
    Insecure    bool
    SSHUser     string
    SSHPassword string
    SSHKeyPath  string
}
```

**Key methods on `Client`:**
- `Connect()` — authenticate (token preferred, password fallback)
- `CreateVM(VMSpec)` — create a VM with cloud-init
- `ListVMs()` — list all VMs on the node
- `DeleteVM(vmid)` — remove a VM
- `DownloadImage(Image)` — download a cloud image to Proxmox storage
- `GetVMIPAddress(vmid)` — wait for and return the VM IP
- `WaitForVMReady(vmid)` — health-check via SSH until ready
- `UploadBinary(vmid, path)` — SCP a local binary to the VM
- `ExecuteBinary(vmid, path, args)` — SSH-execute a binary on the VM

**Built-in images** (via `DefaultImages()`):
- Debian 11 (bullseye)
- Debian 13 (trixie)
- Ubuntu 24.04 LTS (noble)

**`VMSpec` fields**: Name, VMID, Image, Memory, CPU, Cores, CloudInit, Network

### `pkg/cloudconfig` — Cloud-Init Configuration

Generates cloud-init `user-data` YAML for automated VM provisioning.

```go
// Fluent builder pattern
cfg, err := cloudconfig.NewBuilder().
    SetHostname("my-vm").
    AddUser("ubuntu", "pubkey...").
    AddPackage("curl").
    AddRunCommand("apt-get update").
    Build()
yaml, err := cfg.Generate()
```

**Supports**: hostname, users, SSH keys, sudo, packages, run commands, environment variables, shell configuration.

### `pkg/binary` — Binary Utilities

Validates and hashes local binaries before transfer.

```go
info, err := binary.GetBinaryInfo("/path/to/binary")
// info.MD5Hash, info.SHA256Hash, info.Size, info.Mode
err = binary.ValidateBinary("/path/to/binary")  // checks executable bit
err = binary.VerifyBinary("/path/to/binary", expectedMD5, expectedSHA256)
```

### `pkg/ssh` — SSH Client

Thin wrapper around `golang.org/x/crypto/ssh` with password and key auth support.

```go
cfg := ssh.Config{Host: "192.168.1.10", Port: 22, Username: "ubuntu", Password: "..."}
client := ssh.NewClient(cfg)
err := client.Connect()
```

### `parseCloudInitLog` — Log Parser

Regex-based parser that extracts structured data from cloud-init serial output:
- IPv4/IPv6 addresses
- SSH host key fingerprints
- SSH public keys
- Hostname
- Authorized keys

---

## Development Workflows

### Build

```bash
go build ./...
go install ./cmd/dtt    # installs 'dtt' binary to $GOPATH/bin
```

### Test

```bash
go test ./...                   # run all tests
go test -cover ./...            # with coverage
go test ./pkg/proxmox/...       # single package
go test -v -run TestGetBinaryInfo ./pkg/binary/...  # single test
```

### Run Locally

```bash
cp .env.example .env
# edit .env with your Proxmox credentials
source .env
dtt run ./mybinary --args "--flag value"
```

### Environment Variables

All credentials can come from environment variables (see `.env.example`):

| Variable | Description |
|---|---|
| `PROXMOX_HOST` | Proxmox server hostname/IP |
| `PROXMOX_NODE` | Proxmox node name |
| `PROXMOX_PORT` | API port (default 8006) |
| `PROXMOX_USERNAME` | API username |
| `DTT_PROXMOX_PASSWORD` | API password |
| `DTT_PROXMOX_TOKEN_ID` | API token ID (preferred over password) |
| `DTT_PROXMOX_TOKEN_SECRET` | API token secret |
| `DTT_PROXMOX_SSH_USER` | SSH username for Proxmox host |
| `DTT_PROXMOX_SSH_PASSWORD` | SSH password for Proxmox host |

---

## CLI Command Reference

```
dtt run <binary> [flags]        # Upload and run a binary on a new VM
dtt vm list                     # List VMs
dtt vm start <vmid>             # Start a VM
dtt vm stop <vmid>              # Stop a VM
dtt vm shutdown <vmid>          # Graceful shutdown
dtt vm restart <vmid>           # Restart a VM
dtt vm reset <vmid>             # Hard reset a VM
dtt vm get <vmid>               # Get VM details
dtt vm delete <vmid>            # Delete a VM
dtt vm cloudinit <vmid>         # Show cloud-init output
dtt vm monitor <vmid>           # Stream VM console output
dtt image list                  # List available images on Proxmox
dtt image upload <file>         # Upload an image to Proxmox
dtt image download <name>       # Download a cloud image
dtt image rm <name>             # Remove an image from Proxmox
dtt agent list                  # List QEMU agents
dtt agent osinfo <vmid>         # Get OS info via QEMU agent
dtt agent network <vmid>        # Get network info via QEMU agent
dtt agent exec <vmid> <cmd>     # Execute command via QEMU agent
dtt agent exec-status <vmid>    # Check agent exec status
dtt status                      # Show cluster status
dtt completion bash|zsh         # Shell completion scripts
```

Global flags (persistent across all commands):
- `--host` — Proxmox host
- `--node` — Proxmox node name
- `--token-id` — API token ID
- `--token-secret` — API token secret
- `--username` / `--password` — username/password auth

---

## Code Conventions

### File Naming

Command files follow a strict pattern:
```
command_<category>_<action>.go   # e.g., command_vm_list.go, command_image_rm.go
```

### Error Handling

Always wrap errors with context using `fmt.Errorf`:
```go
if err != nil {
    return fmt.Errorf("creating VM: %w", err)
}
```
- No `panic()` in library code
- Propagate errors up the call stack; let the CLI layer handle user-facing output

### Logging

Use `log/slog` for structured logging in library code:
```go
slog.Debug("connecting to proxmox", "host", cfg.Host, "node", cfg.Node)
```
Use `fmt.Println` / `fmt.Printf` only in command handlers for user-facing output.

### Authentication Priority

Always attempt token auth before password auth:
```go
if cfg.TokenID != "" && cfg.TokenSecret != "" {
    // use token
} else {
    // use password
}
```

### Testing

- Use the standard `testing` package only (no external test frameworks)
- Test files live alongside source files: `foo.go` / `foo_test.go`
- Test function names: `TestFunctionName` or `TestFunctionNameScenario`
- Use table-driven tests for multiple input cases
- No integration tests requiring a live Proxmox server in the test suite

### Adding a New Command

1. Create `cmd/dtt/command_<category>_<action>.go`
2. Define a function `New<Category><Action>Command() *cobra.Command`
3. Register it in `cmd/dtt/commands/commands.go` under the appropriate parent command
4. Add tests in `cmd/dtt/commands/commands_test.go`

### Adding a New Package

1. Create `pkg/<name>/` directory
2. Package name should match directory name
3. Add `<name>_test.go` alongside
4. Keep packages focused on a single responsibility

---

## Dependencies

| Dependency | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/luthermonson/go-proxmox` | Proxmox REST API client |
| `golang.org/x/crypto` | SSH client support |
| `github.com/gorilla/websocket` | WebSocket (used by go-proxmox) |
| `github.com/diskfs/go-diskfs` | Disk image handling |
| `github.com/magefile/mage` | Build automation (indirect) |

Add new dependencies with `go get <package>` and commit both `go.mod` and `go.sum`.

---

## Architecture Notes

- **DTT as a library**: The `pkg/` packages are designed for import into other Go programs, not just the CLI. Keep them free of CLI-specific concerns.
- **Ephemeral VMs**: The primary use case is short-lived VMs created for a single binary run. Code should assume VMs may be deleted immediately after use.
- **Cloud-init for provisioning**: All VM setup (SSH keys, users, packages) is done via cloud-init at boot — no post-boot configuration management.
- **No persistent state**: DTT has no local database or state file. All state is queried from the Proxmox API at runtime.
- **Insecure TLS option**: The `Insecure` flag in `ClientConfig` skips TLS verification — acceptable for homelab/internal Proxmox servers, should not be used in production.

---

## Known Limitations / TODOs

- No CI/CD pipeline configured (no `.github/workflows/`)
- No Makefile for build automation
- Integration tests require a live Proxmox environment and are not automated
- Only three built-in cloud images (Debian 11, Debian 13, Ubuntu 24.04)
- SSH host key verification uses `InsecureIgnoreHostKey` (acceptable for ephemeral VMs)
