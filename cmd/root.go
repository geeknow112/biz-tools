package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version info (set via ldflags at build time)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "biz-tools",
	Short: "Business automation CLI tool",
	Long: `biz-tools is a CLI tool for automating business workflows.

Available commands:
  media  - Media publishing (Zenn, Qiita, note, WordPress, X)
  video  - Video creation workflow
  fba    - FBA product search and operations`,
	Run: func(cmd *cobra.Command, args []string) {
		showVersion, _ := cmd.Flags().GetBool("version")
		if showVersion {
			fmt.Printf("biz-tools version %s (built %s)\n", Version, BuildTime)
			return
		}
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
}
