package commands
  
		func command_run(cmd *cobra.Command, args []string) error {
			binaryPath := args[0]
			vmID := args[1]

			// Get flags
			hostname, _ := cmd.Flags().GetString("hostname")
			image, _ := cmd.Flags().GetString("image")
			memory, _ := cmd.Flags().GetInt("memory")
			cpu, _ := cmd.Flags().GetInt("cpu")
			cores, _ := cmd.Flags().GetInt("cores")
			username, _ := cmd.Flags().GetString("username")
			remotePath, _ := cmd.Flags().GetString("remote-path")
			vmIP, _ := cmd.Flags().GetString("vm-ip")
			sshPassword, _ := cmd.Flags().GetString("ssh-password")

			// Check environment variable for SSH password if not provided
			if sshPassword == "" {
				sshPassword = os.Getenv("DTT_SSH_PASSWORD")
			}

			// Validate binary
			binInfo, err := binary.GetBinaryInfo(binaryPath)
			if err != nil {
				return fmt.Errorf("failed to validate binary: %w", err)
			}

			fmt.Printf("Binary: %s (%d bytes)\n", binInfo.Name, binInfo.Size)
			fmt.Printf("SHA256: %s\n", binInfo.SHA256Hash)

			// Get Proxmox client
			client := getProxmoxClient(cmd)

			// Select image
			selectedImage, err := selectImage(image)
			if err != nil {
				return err
			}

			fmt.Printf("Selected image: %s\n", selectedImage.Name)

			// Create cloud-init config (for future use)
			_ = cloudconfig.NewBuilder().
				WithHostname(hostname).
				WithUsername(username).
				WithPackage("curl").
				Build()

			// Create VM spec
			vmSpec := proxmox.VMSpec{
				Name:      hostname,
				VMID:      parseVMID(vmID),
				Image:     selectedImage,
				Memory:    memory,
				CPU:       cpu,
				Cores:     cores,
				CloudInit: true,
			}

			fmt.Printf("Creating VM: %s (ID: %d)\n", vmSpec.Name, vmSpec.VMID)

			// Create the VM
			vm, err := client.CreateVM(vmSpec)
			if err != nil {
				return fmt.Errorf("failed to create VM: %w", err)
			}

			fmt.Printf("VM created with ID: %d\n", vm.ID)

			// Try to get VM IP if not provided
			if vmIP == "" {
				fmt.Printf("Waiting for VM to get an IP address...\n")
				// Try up to 60 times (5 minutes) to get IP
				for i := 0; i < 60; i++ {
					ip, err := client.GetVMIPAddress(parseVMID(vmID))
					if err == nil && ip != "" {
						vmIP = ip
						fmt.Printf("VM IP address: %s\n", vmIP)
						break
					}
					if i < 59 {
						fmt.Printf("Waiting for VM to boot and get IP... (%d/60)\n", i+1)
						time.Sleep(5 * time.Second)
					}
				}

				if vmIP == "" {
					fmt.Printf("Unable to automatically detect VM IP address.\n")
					fmt.Printf("Please provide --vm-ip flag or check VM network configuration.\n")
					return nil
				}
			}

			// Upload and execute binary
			fmt.Printf("Waiting for VM to be ready at %s...\n", vmIP)
			if err := client.WaitForVMReady(vmIP, username, sshPassword, 30); err != nil {
				return fmt.Errorf("VM did not become ready: %w", err)
			}

			fmt.Printf("Uploading binary to %s on VM...\n", remotePath)
			if err := client.UploadBinary(vmIP, username, sshPassword, binaryPath, remotePath); err != nil {
				return fmt.Errorf("failed to upload binary: %w", err)
			}

			fmt.Printf("Executing binary on VM...\n")
			output, err := client.ExecuteBinary(vmIP, username, sshPassword, remotePath)
			if err != nil {
				fmt.Printf("Binary execution failed: %v\n", err)
				if output != "" {
					fmt.Printf("Output:\n%s\n", output)
				}
				return err
			}

			fmt.Printf("Binary executed successfully!\n")
			if output != "" {
				fmt.Printf("Output:\n%s\n", output)
			}

			return nil
		},