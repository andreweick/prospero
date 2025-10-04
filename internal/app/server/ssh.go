package server

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/muesli/termenv"
	cryptossh "golang.org/x/crypto/ssh"

	"prospero/internal/features/shakespert"
	"prospero/internal/features/topten"
)

// StartSSHServer starts the SSH server with the given host and port
func StartSSHServer(ctx context.Context, host, port string) error {
	// Initialize the topten service
	toptenService, err := topten.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize topten service: %w", err)
	}

	// Initialize the shakespert service
	shakespertService, err := shakespert.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize shakespert service: %w", err)
	}
	defer shakespertService.Close()

	// Decrypt the SSH host key
	hostKey, err := topten.DecryptSSHHostKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to decrypt SSH host key: %w", err)
	}

	// Extract and display the public key and fingerprint
	publicKey, fingerprint, err := extractPublicKeyFromPrivate(hostKey)
	if err != nil {
		fmt.Printf("Warning: failed to extract public key: %v\r\n", err)
		publicKey = "[unable to extract public key]"
		fingerprint = "[unable to extract fingerprint]"
	}

	// Create the SSH server
	server, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", host, port)),
		wish.WithHostKeyPEM(hostKey),
		wish.WithMiddleware(
			prosperoMiddleware(toptenService, shakespertService),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	// Display startup information
	fmt.Printf("\r\n🎩 Prospero SSH Server Starting\r\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\r\n")
	fmt.Printf("📡 Server Address: %s:%s\r\n", host, port)
	fmt.Printf("🔑 Host Key: %s\r\n", publicKey)
	fmt.Printf("🔐 Fingerprint: %s\r\n", fingerprint)
	fmt.Printf("\r\n💻 Connect from:\r\n")

	addresses := getNetworkAddresses(port)
	for _, addr := range addresses {
		fmt.Printf("   %s\r\n", addr)
	}

	fmt.Printf("\r\n🎯 Interactive Prospero SSH Server!\r\n")
	fmt.Printf("📚 Available commands: topten, shakespert, info\r\n")
	fmt.Printf("💡 Try: ssh localhost -p %s info --color\r\n", port)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\r\n")
	fmt.Printf("Server ready. Press Ctrl+C to stop.\r\n\r\n")

	// Start server in a goroutine so we can handle context cancellation
	go func() {
		<-ctx.Done()
		fmt.Printf("🔌 Shutting down SSH server...\r\n")

		// Give server 5 seconds to shut down gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("SSH server forced to shutdown: %v\n", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
		return err
	}

	return nil
}

func prosperoMiddleware(toptenService *topten.Service, shakespertService *shakespert.Service) wish.Middleware {
	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			// Get command from SSH session command
			cmd := s.Command()
			if len(cmd) == 0 {
				// Default behavior - show help
				showSSHHelp(s)
			} else {
				command := strings.ToLower(cmd[0])
				switch command {
				case "topten", "top-ten":
					handleTopTenSSH(s, toptenService)
				case "shakespert", "shakespeare", "works":
					handleShakespertSSH(s, shakespertService, cmd[1:])
				case "info":
					handleInfoSSH(s)
				default:
					fmt.Fprintf(s, "Unknown command: %s\n\n", command)
					showSSHHelp(s)
				}
			}

			// End the session
			sh(s)
		}
	}
}

func showSSHHelp(s ssh.Session) {
	fmt.Fprintf(s, "\n🎩 Welcome to Prospero SSH Server!\n")
	fmt.Fprintf(s, "═════════════════════════════════════\n\n")
	fmt.Fprintf(s, "Available commands:\n")
	fmt.Fprintf(s, "  topten [--color|--ascii]  - Get a random David Letterman Top 10 list\n")
	fmt.Fprintf(s, "  shakespert works          - List all Shakespeare works\n")
	fmt.Fprintf(s, "  shakespert work ID        - Show details for a specific work\n")
	fmt.Fprintf(s, "  shakespert genres         - List all genres\n")
	fmt.Fprintf(s, "  info [--color|--ascii]    - Show detailed server information\n")
	fmt.Fprintf(s, "\nFlags:\n")
	fmt.Fprintf(s, "  --color  - Use fancy colored output\n")
	fmt.Fprintf(s, "  --ascii  - Use plain text output (default)\n")
	fmt.Fprintf(s, "\nExamples:\n")
	fmt.Fprintf(s, "  ssh user@host -p 2222 topten --color\n")
	fmt.Fprintf(s, "  ssh user@host -p 2222 shakespert works\n")
	fmt.Fprintf(s, "  ssh user@host -p 2222 shakespert work hamlet\n")
	fmt.Fprintf(s, "  ssh user@host -p 2222 info --color\n")
	fmt.Fprintf(s, "\n")
}

func handleTopTenSSH(s ssh.Session, service *topten.Service) {
	// Parse flags from command arguments
	useColor := false
	cmd := s.Command()
	for i := 1; i < len(cmd); i++ {
		if cmd[i] == "--color" {
			useColor = true
		} else if cmd[i] == "--ascii" {
			useColor = false
		}
	}

	// Set color profile based on flag (default to ASCII)
	if useColor {
		lipgloss.SetColorProfile(termenv.TrueColor)
	} else {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	list, err := service.GetRandomList()
	if err != nil {
		fmt.Fprintf(s, "Error getting random list: %v\n", err)
		return
	}

	// Print the list using the appropriate formatting
	if useColor {
		topten.PrintList(s, list)
	} else {
		topten.PrintListASCII(s, list)
	}
}

func handleInfoSSH(s ssh.Session) {
	// Parse flags from command arguments
	useColor := false
	cmd := s.Command()
	for i := 1; i < len(cmd); i++ {
		if cmd[i] == "--color" {
			useColor = true
		} else if cmd[i] == "--ascii" {
			useColor = false
		}
	}

	// Set color profile based on flag (default to ASCII)
	if useColor {
		lipgloss.SetColorProfile(termenv.TrueColor)
	} else {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	// Define styles based on color mode
	var titleStyle, sectionStyle, commandStyle, exampleStyle, containerStyle lipgloss.Style

	if useColor {
		titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2).
			Margin(1, 0)

		sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			Margin(1, 0, 0, 0)

		commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4"))

		exampleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#95E1D3")).
			Italic(true)

		containerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Margin(1, 0)
	} else {
		titleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 2).
			Margin(1, 0)

		sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Margin(1, 0, 0, 0)

		commandStyle = lipgloss.NewStyle()

		exampleStyle = lipgloss.NewStyle().
			Italic(true)

		containerStyle = lipgloss.NewStyle().
			Border(lipgloss.ASCIIBorder()).
			Padding(1, 2).
			Margin(1, 0)
	}

	// Build content
	var content strings.Builder

	content.WriteString(titleStyle.Render("🎩 Prospero SSH Server"))
	content.WriteString("\n\n")
	content.WriteString("An interactive SSH server for exploring classic literature and entertainment.\n\n")

	content.WriteString(sectionStyle.Render("Available Commands:"))
	content.WriteString("\n\n")
	content.WriteString(commandStyle.Render("  topten [--color|--ascii]"))
	content.WriteString("\n")
	content.WriteString("    Display a random David Letterman Top 10 list\n\n")
	content.WriteString(commandStyle.Render("  shakespert works"))
	content.WriteString("\n")
	content.WriteString("    List all Shakespeare works\n\n")
	content.WriteString(commandStyle.Render("  shakespert work <id>"))
	content.WriteString("\n")
	content.WriteString("    Show details for a specific work\n\n")
	content.WriteString(commandStyle.Render("  shakespert genres"))
	content.WriteString("\n")
	content.WriteString("    List all genres\n\n")
	content.WriteString(commandStyle.Render("  info [--color|--ascii]"))
	content.WriteString("\n")
	content.WriteString("    Show this information page\n\n")

	content.WriteString(sectionStyle.Render("💡 Tips:"))
	content.WriteString("\n\n")
	content.WriteString("  • Use ")
	content.WriteString(commandStyle.Render("--color"))
	content.WriteString(" flag for fancy colored output\n")
	content.WriteString("  • Use ")
	content.WriteString(commandStyle.Render("--ascii"))
	content.WriteString(" flag for plain text output (default)\n")
	content.WriteString("  • Most modern terminals support colored output\n\n")

	content.WriteString(sectionStyle.Render("Examples:"))
	content.WriteString("\n\n")
	content.WriteString(exampleStyle.Render("  ssh localhost -p 2222 topten --color"))
	content.WriteString("\n")
	content.WriteString(exampleStyle.Render("  ssh localhost -p 2222 shakespert works"))
	content.WriteString("\n")
	content.WriteString(exampleStyle.Render("  ssh localhost -p 2222 shakespert work hamlet"))
	content.WriteString("\n")
	content.WriteString(exampleStyle.Render("  ssh localhost -p 2222 info --color"))
	content.WriteString("\n")

	// Apply container and print
	finalOutput := containerStyle.Render(content.String())
	fmt.Fprintf(s, "\n%s\n\n", finalOutput)
}

func handleShakespertSSH(s ssh.Session, service *shakespert.Service, args []string) {
	ctx := s.Context()

	if len(args) == 0 {
		fmt.Fprintf(s, "shakespert command requires a subcommand. Use 'works', 'work <id>', or 'genres'\n")
		return
	}

	subcommand := strings.ToLower(args[0])
	switch subcommand {
	case "works":
		works, err := service.ListWorks(ctx)
		if err != nil {
			fmt.Fprintf(s, "Error listing works: %v\n", err)
			return
		}

		fmt.Fprintf(s, "\n📚 Shakespeare's Complete Works (%d works)\n", len(works))
		fmt.Fprintf(s, "%s\n\n", strings.Repeat("═", 50))

		currentGenre := ""
		for _, work := range works {
			if work.GenreName != currentGenre {
				if currentGenre != "" {
					fmt.Fprintf(s, "\n")
				}
				fmt.Fprintf(s, "%s:\n", work.GenreName)
				fmt.Fprintf(s, "%s\n", strings.Repeat("─", len(work.GenreName)+1))
				currentGenre = work.GenreName
			}

			yearStr := ""
			if work.Date > 0 {
				yearStr = fmt.Sprintf(" (%d)", work.Date)
			}

			fmt.Fprintf(s, "  %s - %s%s\n", work.WorkID, work.Title, yearStr)
		}
		fmt.Fprintf(s, "\n")

	case "work":
		if len(args) < 2 {
			fmt.Fprintf(s, "work command requires a work ID. Example: shakespert work hamlet\n")
			return
		}

		workID := args[1]
		work, err := service.GetWork(ctx, workID)
		if err != nil {
			fmt.Fprintf(s, "Error getting work: %v\n", err)
			return
		}

		fmt.Fprintf(s, "\n📖 %s\n", work.Title)
		fmt.Fprintf(s, "%s\n", strings.Repeat("═", len(work.Title)+4))

		if work.LongTitle != work.Title && work.LongTitle != "" {
			fmt.Fprintf(s, "Full Title: %s\n", work.LongTitle)
		}

		fmt.Fprintf(s, "Work ID: %s\n", work.WorkID)
		fmt.Fprintf(s, "Genre: %s (%s)\n", work.GenreName, work.GenreType)

		if work.Date > 0 {
			fmt.Fprintf(s, "Year: %d\n", work.Date)
		}

		fmt.Fprintf(s, "Words: %d\n", work.TotalWords)
		fmt.Fprintf(s, "Paragraphs: %d\n", work.TotalParagraphs)

		if work.Source != "" {
			fmt.Fprintf(s, "Source: %s\n", work.Source)
		}

		fmt.Fprintf(s, "\n")

	case "genres":
		genres, err := service.ListGenres(ctx)
		if err != nil {
			fmt.Fprintf(s, "Error listing genres: %v\n", err)
			return
		}

		fmt.Fprintf(s, "\n📚 Shakespeare Genres\n")
		fmt.Fprintf(s, "%s\n\n", strings.Repeat("═", 18))

		for _, genre := range genres {
			genreName := genre.Genrename.String
			if !genre.Genrename.Valid {
				genreName = ""
			}
			fmt.Fprintf(s, "%s - %s\n", genre.Genretype, genreName)
		}
		fmt.Fprintf(s, "\n")

	default:
		fmt.Fprintf(s, "Unknown shakespert subcommand: %s\n", subcommand)
		fmt.Fprintf(s, "Available subcommands: works, work <id>, genres\n")
	}
}

// extractPublicKeyFromPrivate extracts the SSH public key and fingerprint from an OpenSSH private key
func extractPublicKeyFromPrivate(privateKeyPEM []byte) (string, string, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return "", "", fmt.Errorf("failed to decode PEM block")
	}

	// Parse the private key
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try OpenSSH format
		privateKey, err = cryptossh.ParseRawPrivateKey(privateKeyPEM)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	// Extract the public key based on type
	var publicKey interface{}
	switch priv := privateKey.(type) {
	case *ed25519.PrivateKey:
		publicKey = priv.Public()
	case ed25519.PrivateKey:
		publicKey = priv.Public()
	default:
		return "", "", fmt.Errorf("unsupported key type: %T", privateKey)
	}

	// Convert to SSH public key format
	sshPublicKey, err := cryptossh.NewPublicKey(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Format as SSH public key string
	publicKeyStr := fmt.Sprintf("%s %s prospero-ssh-server",
		sshPublicKey.Type(),
		base64.StdEncoding.EncodeToString(sshPublicKey.Marshal()))

	// Compute fingerprint
	fingerprint := cryptossh.FingerprintSHA256(sshPublicKey)

	return publicKeyStr, fingerprint, nil
}

// getLocalIP gets the local IP address that would be used for outbound connections
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "localhost"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// getNetworkAddresses returns a list of network addresses the server can be reached on
func getNetworkAddresses(port string) []string {
	addresses := []string{}

	// Add localhost
	addresses = append(addresses, fmt.Sprintf("ssh localhost -p %s", port))

	// Get local IP
	localIP := getLocalIP()
	if localIP != "localhost" {
		addresses = append(addresses, fmt.Sprintf("ssh %s -p %s", localIP, port))
	}

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}

				if ip == nil || ip.IsLoopback() {
					continue
				}

				// Only include IPv4 addresses to avoid clutter
				if ip.To4() != nil && ip.String() != localIP {
					addresses = append(addresses, fmt.Sprintf("ssh %s -p %s", ip.String(), port))
				}
			}
		}
	}

	return addresses
}
