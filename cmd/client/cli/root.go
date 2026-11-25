package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	serverAddr    string
	subdomain     string
	token         string
	inspectorPort int
)

var rootCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "tunnel - A fast, self-hosted reverse proxy and tunneling engine in Go",
	Long: `tunnel is a self-hosted ngrok alternative that creates secure,
multiplexed reverse tunnels from public endpoints to your local development machine.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// GetRootCmd exposes rootCmd for testing purposes.
func GetRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "localhost:9000", "Tunnel server control address")
	rootCmd.PersistentFlags().StringVarP(&subdomain, "subdomain", "d", "gaurav", "Custom subdomain request")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "t", "dev-secret-token", "Authentication token")
	rootCmd.PersistentFlags().IntVar(&inspectorPort, "inspector", 4040, "Local inspector port")
}

