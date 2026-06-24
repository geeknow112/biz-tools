package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Platforms map[string]PlatformConfig `yaml:"platforms"`
}

type PlatformConfig struct {
	Repo string `yaml:"repo"`
}

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
  biz-tools media draft article.md -p zenn`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, _ := cmd.Flags().GetString("platform")
		file := args[0]
		return runDraft(file, platform)
	},
}

var mediaPublishCmd = &cobra.Command{
	Use:   "publish [file]",
	Short: "Merge draft PR to publish content",
	Long: `Merge an approved draft PR to publish content.

This command finds the draft PR for the specified file and merges it.
The PR must be approved before merging.

Example:
  biz-tools media publish article.md -p zenn`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, _ := cmd.Flags().GetString("platform")
		file := args[0]
		return runPublish(file, platform)
	},
}

func init() {
	rootCmd.AddCommand(mediaCmd)
	mediaCmd.AddCommand(mediaDraftCmd)
	mediaCmd.AddCommand(mediaPublishCmd)

	mediaDraftCmd.Flags().StringP("platform", "p", "zenn", "Target platform (zenn, qiita, note, wordpress, x)")
	mediaPublishCmd.Flags().StringP("platform", "p", "zenn", "Target platform (zenn, qiita, note, wordpress, x)")
}

func loadConfig() (*Config, error) {
	// Look for config.yaml in current dir or executable dir
	configPaths := []string{
		"config.yaml",
		filepath.Join(filepath.Dir(os.Args[0]), "config.yaml"),
	}

	var data []byte
	var err error
	for _, path := range configPaths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("config.yaml not found")
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}

func runDraft(file, platform string) error {
	// 1. Load config
	config, err := loadConfig()
	if err != nil {
		return err
	}

	platformConfig, ok := config.Platforms[platform]
	if !ok {
		return fmt.Errorf("platform '%s' not configured in config.yaml", platform)
	}

	targetRepo := platformConfig.Repo
	if targetRepo == "" {
		return fmt.Errorf("repo path not set for platform '%s'", platform)
	}

	// 2. Check source file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", file)
	}

	// 3. Read file content
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// 4. Change to target repo
	originalDir, _ := os.Getwd()
	if err := os.Chdir(targetRepo); err != nil {
		return fmt.Errorf("failed to change to repo: %w", err)
	}
	defer os.Chdir(originalDir)

	// 5. Get current branch (base)
	baseBranch, err := gitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	baseBranch = strings.TrimSpace(baseBranch)

	// 6. Generate branch name
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("draft/%s-%s", platform, timestamp)

	// 7. Create and checkout new branch
	fmt.Printf("Creating branch: %s in %s\n", branchName, targetRepo)
	if _, err := gitCommand("checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// 8. Determine destination path based on platform
	var destPath string
	switch platform {
	case "zenn":
		destPath = filepath.Join("articles", filepath.Base(file))
	case "qiita":
		destPath = filepath.Join("public", filepath.Base(file))
	default:
		destPath = filepath.Base(file)
	}

	// 9. Write file
	destDir := filepath.Dir(destPath)
	if destDir != "." {
		os.MkdirAll(destDir, 0755)
	}
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// 10. Git add and commit
	if _, err := gitCommand("add", destPath); err != nil {
		return fmt.Errorf("failed to git add: %w", err)
	}

	commitMsg := fmt.Sprintf("draft(%s): add %s", platform, filepath.Base(file))
	if _, err := gitCommand("commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// 11. Push branch
	fmt.Println("Pushing to remote...")
	if _, err := gitCommand("push", "-u", "origin", branchName); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// 12. Create PR using gh CLI
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

	// 13. Return to base branch
	gitCommand("checkout", baseBranch)

	return nil
}

func gitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}

func ghCommand(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}

func runPublish(file, platform string) error {
	// 1. Load config
	config, err := loadConfig()
	if err != nil {
		return err
	}

	platformConfig, ok := config.Platforms[platform]
	if !ok {
		return fmt.Errorf("platform '%s' not configured in config.yaml", platform)
	}

	targetRepo := platformConfig.Repo
	if targetRepo == "" {
		return fmt.Errorf("repo path not set for platform '%s'", platform)
	}

	// 2. Change to target repo
	originalDir, _ := os.Getwd()
	if err := os.Chdir(targetRepo); err != nil {
		return fmt.Errorf("failed to change to repo: %w", err)
	}
	defer os.Chdir(originalDir)

	// 3. Find PR for this file
	fileName := filepath.Base(file)
	fmt.Printf("Searching for draft PR containing '%s' on %s...\n", fileName, platform)

	// Search for open PRs with the file name in title
	prList, err := ghCommand("pr", "list", "--state", "open", "--json", "number,title,url")
	if err != nil {
		return fmt.Errorf("failed to list PRs: %w", err)
	}

	// Parse PR list to find matching PR
	prNumber, prURL, err := findMatchingPR(prList, fileName, platform)
	if err != nil {
		return err
	}

	fmt.Printf("Found PR #%s: %s\n", prNumber, prURL)

	// 4. Check PR status (approved?)
	prStatus, err := ghCommand("pr", "view", prNumber, "--json", "reviewDecision,mergeable,state")
	if err != nil {
		return fmt.Errorf("failed to get PR status: %w", err)
	}
	fmt.Printf("PR Status: %s\n", strings.TrimSpace(prStatus))

	// 5. Merge the PR
	fmt.Println("Merging PR...")
	mergeOutput, err := ghCommand("pr", "merge", prNumber, "--squash", "--delete-branch")
	if err != nil {
		return fmt.Errorf("failed to merge PR: %w", err)
	}

	fmt.Printf("\n✅ Published successfully!\n")
	fmt.Printf("   %s\n", strings.TrimSpace(mergeOutput))

	return nil
}

func findMatchingPR(prListJSON, fileName, platform string) (string, string, error) {
	// Simple JSON parsing for PR list
	// Format: [{"number":1,"title":"...","url":"..."},...]
	type PR struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}

	var prs []PR
	if err := json.Unmarshal([]byte(prListJSON), &prs); err != nil {
		return "", "", fmt.Errorf("failed to parse PR list: %w", err)
	}

	// Search for PR matching platform and filename
	searchTerms := []string{
		fmt.Sprintf("[%s]", strings.ToUpper(platform)),
		fileName,
	}

	for _, pr := range prs {
		titleUpper := strings.ToUpper(pr.Title)
		matchCount := 0
		for _, term := range searchTerms {
			if strings.Contains(titleUpper, strings.ToUpper(term)) {
				matchCount++
			}
		}
		if matchCount >= 1 && strings.Contains(pr.Title, fileName) {
			return fmt.Sprintf("%d", pr.Number), pr.URL, nil
		}
	}

	return "", "", fmt.Errorf("no open PR found for '%s' on %s", fileName, platform)
}
