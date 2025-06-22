// internal/network/onboarding.go
package network

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

var (
	successColor = color.New(color.FgGreen).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	warningColor = color.New(color.FgYellow).SprintFunc()
	infoColor    = color.New(color.FgCyan).SprintFunc()
)

// TunnelOnboarding handles first-time tunnel setup
type TunnelOnboarding struct {
	provider string
}

// NewTunnelOnboarding creates a new onboarding helper
func NewTunnelOnboarding() *TunnelOnboarding {
	return &TunnelOnboarding{}
}

// CheckAndSetup checks if tunnel provider is ready and sets it up if needed
func (t *TunnelOnboarding) CheckAndSetup() (string, error) {
	// Check if cloudflared is installed
	cloudflaredInstalled := t.isCloudflaredInstalled()

	// Check if ngrok token is available
	ngrokReady := os.Getenv("NGROK_AUTH_TOKEN") != ""

	// If cloudflared is installed, use it
	if cloudflaredInstalled {
		t.provider = "cloudflare"
		return "cloudflare", nil
	}

	// If ngrok is ready, use it
	if ngrokReady {
		t.provider = "ngrok"
		return "ngrok", nil
	}

	// Neither is ready, start onboarding
	fmt.Println(warningColor("No tunnel provider is configured. Let's set one up!"))
	fmt.Println()

	return t.runOnboarding()
}

// runOnboarding runs the interactive setup
func (t *TunnelOnboarding) runOnboarding() (string, error) {
	fmt.Println("LocalCloud needs a tunnel provider to create public URLs for your services.")
	fmt.Println()
	fmt.Println("Available options:")
	fmt.Println("1. " + infoColor("Cloudflare Tunnel") + " (Recommended)")
	fmt.Println("   ✓ Completely free")
	fmt.Println("   ✓ No account required for quick tunnels")
	fmt.Println("   ✓ Stable URLs with free account")
	fmt.Println()
	fmt.Println("2. " + infoColor("Ngrok"))
	fmt.Println("   ✓ Easy setup")
	fmt.Println("   ✗ Requires free account")
	fmt.Println("   ✗ Random URLs on free tier")
	fmt.Println()
	fmt.Print("Which would you like to use? [1-2, default: 1]: ")

	var choice string
	fmt.Scanln(&choice)

	if choice == "" || choice == "1" {
		return t.setupCloudflare()
	} else if choice == "2" {
		return t.setupNgrok()
	}

	return "", fmt.Errorf("invalid choice")
}

// setupCloudflare guides through Cloudflare setup
func (t *TunnelOnboarding) setupCloudflare() (string, error) {
	fmt.Println()
	fmt.Println(infoColor("Setting up Cloudflare Tunnel..."))

	// Check if already installed
	if t.isCloudflaredInstalled() {
		fmt.Println(successColor("✓ cloudflared is already installed!"))
		t.provider = "cloudflare"
		return "cloudflare", nil
	}

	// Provide installation instructions
	fmt.Println()
	fmt.Println("cloudflared is not installed. Here's how to install it:")
	fmt.Println()

	switch runtime.GOOS {
	case "darwin":
		fmt.Println(infoColor("macOS:"))
		fmt.Println("  brew install cloudflared")
		fmt.Println()
		fmt.Println("Or download from:")
		fmt.Println("  https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/")

		// Offer to install automatically
		fmt.Println()
		fmt.Print("Would you like to install it now using Homebrew? [Y/n]: ")
		var install string
		fmt.Scanln(&install)

		if install == "" || strings.ToLower(install) == "y" {
			return t.installCloudflaredMac()
		}

	case "linux":
		fmt.Println(infoColor("Linux:"))
		t.showLinuxInstructions()

	case "windows":
		fmt.Println(infoColor("Windows:"))
		fmt.Println("Download the installer from:")
		fmt.Println("  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.msi")
		fmt.Println()
		fmt.Println("Or using winget:")
		fmt.Println("  winget install --id Cloudflare.cloudflared")
	}

	fmt.Println()
	fmt.Println("After installation, run this command again.")
	return "", fmt.Errorf("cloudflared not installed")
}

// setupNgrok guides through Ngrok setup
func (t *TunnelOnboarding) setupNgrok() (string, error) {
	fmt.Println()
	fmt.Println(infoColor("Setting up Ngrok..."))

	// Check if token exists
	if token := os.Getenv("NGROK_AUTH_TOKEN"); token != "" {
		fmt.Println(successColor("✓ Ngrok auth token found!"))
		t.provider = "ngrok"
		return "ngrok", nil
	}

	fmt.Println()
	fmt.Println("Ngrok requires a free account to get an auth token.")
	fmt.Println()
	fmt.Println("Steps:")
	fmt.Println("1. Sign up at: " + infoColor("https://dashboard.ngrok.com/signup"))
	fmt.Println("2. Get your auth token from: " + infoColor("https://dashboard.ngrok.com/get-started/your-authtoken"))
	fmt.Println("3. Set the token:")
	fmt.Println()

	switch runtime.GOOS {
	case "windows":
		fmt.Println("   Command Prompt:")
		fmt.Println("   set NGROK_AUTH_TOKEN=your_token_here")
		fmt.Println()
		fmt.Println("   PowerShell:")
		fmt.Println("   $env:NGROK_AUTH_TOKEN=\"your_token_here\"")
	default:
		fmt.Println("   export NGROK_AUTH_TOKEN=your_token_here")
		fmt.Println()
		fmt.Println("   Or add to your shell profile (~/.bashrc, ~/.zshrc, etc.):")
		fmt.Println("   echo 'export NGROK_AUTH_TOKEN=your_token_here' >> ~/.zshrc")
	}

	fmt.Println()
	fmt.Print("Have you set the NGROK_AUTH_TOKEN? [y/N]: ")
	var ready string
	fmt.Scanln(&ready)

	if strings.ToLower(ready) == "y" {
		// Re-check token
		if token := os.Getenv("NGROK_AUTH_TOKEN"); token != "" {
			fmt.Println(successColor("✓ Token detected!"))
			t.provider = "ngrok"
			return "ngrok", nil
		}
		fmt.Println(errorColor("Token not found. Make sure to set it in the current shell."))
	}

	return "", fmt.Errorf("ngrok not configured")
}

// isCloudflaredInstalled checks if cloudflared is in PATH
func (t *TunnelOnboarding) isCloudflaredInstalled() bool {
	_, err := exec.LookPath("cloudflared")
	return err == nil
}

// installCloudflaredMac attempts to install cloudflared on macOS
func (t *TunnelOnboarding) installCloudflaredMac() (string, error) {
	// Check if Homebrew is installed
	if _, err := exec.LookPath("brew"); err != nil {
		fmt.Println(errorColor("Homebrew is not installed."))
		fmt.Println("Install it from: https://brew.sh")
		return "", fmt.Errorf("homebrew not found")
	}

	fmt.Println("Installing cloudflared...")
	cmd := exec.Command("brew", "install", "cloudflared")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("installation failed: %w", err)
	}

	fmt.Println(successColor("✓ cloudflared installed successfully!"))
	t.provider = "cloudflare"
	return "cloudflare", nil
}

// showLinuxInstructions shows distro-specific instructions
func (t *TunnelOnboarding) showLinuxInstructions() {
	fmt.Println("For Debian/Ubuntu:")
	fmt.Println("  # Add cloudflare gpg key")
	fmt.Println("  sudo mkdir -p --mode=0755 /usr/share/keyrings")
	fmt.Println("  curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null")
	fmt.Println()
	fmt.Println("  # Add repo")
	fmt.Println("  echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared focal main' | sudo tee /etc/apt/sources.list.d/cloudflared.list")
	fmt.Println()
	fmt.Println("  # Install")
	fmt.Println("  sudo apt-get update && sudo apt-get install cloudflared")
	fmt.Println()
	fmt.Println("For RHEL/CentOS:")
	fmt.Println("  curl -fsSL https://pkg.cloudflare.com/cloudflare-ascii.repo | sudo tee /etc/yum.repos.d/cloudflared.repo")
	fmt.Println("  sudo yum install cloudflared")
	fmt.Println()
	fmt.Println("For Arch Linux:")
	fmt.Println("  yay -S cloudflared")
	fmt.Println()
	fmt.Println("Or download binary:")
	fmt.Println("  https://github.com/cloudflare/cloudflared/releases/latest")
}

// GetConfiguredProvider returns the configured provider after onboarding
func (t *TunnelOnboarding) GetConfiguredProvider() string {
	return t.provider
}

// IsProviderReady checks if a specific provider is ready to use
func IsProviderReady(provider string) bool {
	switch provider {
	case "cloudflare":
		_, err := exec.LookPath("cloudflared")
		return err == nil
	case "ngrok":
		return os.Getenv("NGROK_AUTH_TOKEN") != ""
	default:
		return false
	}
}
