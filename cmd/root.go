package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "biz-tools",
	Short: "Business automation CLI tool",
	Long: `biz-tools is a CLI tool for automating business workflows.

Available commands:
  media  - Media publishing (Zenn, Qiita, note, WordPress, X)
  video  - Video creation workflow
  fba    - FBA product search and operations`,
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
