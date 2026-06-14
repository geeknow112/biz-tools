package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	Long: `Create a draft article and submit a Pull Request for review.

Example:
  biz-tools media draft article.md -p zenn
  biz-tools media draft ./posts/my-article.md -p qiita`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, _ := cmd.Flags().GetString("platform")
		file := args[0]
		return runDraft(file, platform)
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

func runDraft(file, platform string) error {
	ctx := context.Background()
	_ = ctx // Reserved for future GitHub API use

	// 1. Check file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", file)
	}

	// 2. Read file content
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// 3. Generate branch name
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("draft/%s-%s", platform, timestamp)

	// 4. Get current branch
	baseBranch, err := gitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	baseBranch = strings.TrimSpace(baseBranch)

	// 5. Create and checkout new branch
	fmt.Printf("Creating branch: %s\n", branchName)
	if _, err := gitCommand("checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// 6. Determine destination path
	destDir := fmt.Sprintf("articles/%s", platform)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	destPath := filepath.Join(destDir, filepath.Base(file))
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// 7. Git add and commit
	if _, err := gitCommand("add", destPath); err != nil {
		return fmt.Errorf("failed to git add: %w", err)
	}

	commitMsg := fmt.Sprintf("draft(%s): add %s", platform, filepath.Base(file))
	if _, err := gitCommand("commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// 8. Push branch
	fmt.Println("Pushing to remote...")
	if _, err := gitCommand("push", "-u", "origin", branchName); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// 9. Create PR using gh CLI
	fmt.Println("Creating Pull Request...")
	prTitle := fmt.Sprintf("[%s] %s", strings.ToUpper(platform), filepath.Base(file))
	prBody := fmt.Sprintf("## Draft Article\n\n- Platform: %s\n- File: %s\n\nPlease review and approve to publish.", platform, filepath.Base(file))

	prURL, err := ghCommand("pr", "create",
		"--title", prTitle,
		"--body", prBody,
		"--base", baseBranch,
		"--head", branchName)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	fmt.Printf("\n✅ Draft PR created successfully!\n")
	fmt.Printf("   PR URL: %s\n", strings.TrimSpace(prURL))

	// 10. Return to base branch
	gitCommand("checkout", baseBranch)

	return nil
}

func gitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ghCommand(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
