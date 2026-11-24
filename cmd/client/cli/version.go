package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "v1.0.0"
	BuildDate = "2026-11"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and build commit of tunnel",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tunnel version %s (built %s)\n", Version, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

