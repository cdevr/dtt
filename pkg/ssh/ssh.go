package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config contains SSH connection configuration
type Config struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string
	Timeout    time.Duration
}

// Client represents an SSH client connection
type Client struct {
	config     Config
	sshClient  *ssh.Client
	connected  bool
}

// NewClient creates a new SSH client
func NewClient(config Config) *Client {
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &Client{
		config: config,
	}
}

// Connect establishes an SSH connection
func (c *Client) Connect() error {
	if c.connected {
		return nil
	}

	var authMethod ssh.AuthMethod

	if c.config.PrivateKey != "" {
		// Use private key authentication
		key, err := os.ReadFile(c.config.PrivateKey)
		if err != nil {
			return fmt.Errorf("unable to read private key: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("unable to parse private key: %w", err)
		}

		authMethod = ssh.PublicKeys(signer)
	} else {
		// Use password authentication
		authMethod = ssh.Password(c.config.Password)
	}

	sshConfig := &ssh.ClientConfig{
		User: c.config.Username,
		Auth: []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, use proper host key verification
		Timeout:         c.config.Timeout,
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	c.sshClient = client
	c.connected = true
	return nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.sshClient != nil {
		c.connected = false
		return c.sshClient.Close()
	}
	return nil
}

// Execute runs a command on the remote server and returns the output
func (c *Client) Execute(command string) (string, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return "", err
		}
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %w", err)
	}

	return string(output), nil
}

// UploadFile uploads a local file to the remote server using SCP
func (c *Client) UploadFile(localPath, remotePath string) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file info
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create SCP session
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Get stdin pipe
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Start SCP receive command on remote
	go func() {
		defer stdin.Close()

		// Send file header
		fmt.Fprintf(stdin, "C%04o %d %s\n", fileInfo.Mode().Perm(), fileInfo.Size(), filepath.Base(remotePath))

		// Send file content
		io.Copy(stdin, localFile)

		// Send termination byte
		fmt.Fprint(stdin, "\x00")
	}()

	// Execute SCP command
	remoteDir := filepath.Dir(remotePath)
	if err := session.Run(fmt.Sprintf("scp -t %s", remoteDir)); err != nil {
		return fmt.Errorf("scp command failed: %w", err)
	}

	return nil
}

// WaitForConnection retries SSH connection until successful or timeout
func (c *Client) WaitForConnection(maxRetries int, retryDelay time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		err := c.Connect()
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("failed to establish SSH connection after %d attempts", maxRetries)
}
