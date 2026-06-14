package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Media publishing commands",
	Long:  `Commands for publishing content to various platforms (Zenn, Qiita, note, WordPress, X).`,
}

var mediaDraftCmd = &cobra.Command{
	Use:   "draft [file]",
	Short: "Create a draft and PR on GitHub",
	Long:  `Create a draft article and submit a Pull Request for review.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		platform, _ := cmd.Flags().GetString("platform")
		file := args[0]
		fmt.Printf("Creating draft for %s on platform: %s\n", file, platform)
		// TODO: Implement draft creation and PR
	},
}

var mediaPublishCmd = &cobra.Command{
	Use:   "publish [file]",
	Short: "Publish content to platforms",
	Long:  `Publish approved content to specified platforms.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		platform, _ := cmd.Flags().GetString("platform")
		file := args[0]
		fmt.Printf("Publishing %s to platform: %s\n", file, platform)
		// TODO: Implement publishing
	},
}

func init() {
	rootCmd.AddCommand(mediaCmd)
	mediaCmd.AddCommand(mediaDraftCmd)
	mediaCmd.AddCommand(mediaPublishCmd)

	// Flags for draft command
	mediaDraftCmd.Flags().StringP("platform", "p", "zenn", "Target platform (zenn, qiita, note, wordpress, x)")

	// Flags for publish command
	mediaPublishCmd.Flags().StringP("platform", "p", "zenn", "Target platform (zenn, qiita, note, wordpress, x)")
}
