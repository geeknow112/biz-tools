package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

type Tweet struct {
	Text        string `json:"text"`
	ScheduledAt string `json:"scheduled_at,omitempty"`
}

var tweetCmd = &cobra.Command{
	Use:   "tweet",
	Short: "X(Twitter)ツイート管理",
}

var tweetAddCmd = &cobra.Command{
	Use:   "add [text]",
	Short: "ツイートを予約追加",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		scheduledAt, _ := cmd.Flags().GetString("at")

		tweet := Tweet{Text: text}
		if scheduledAt != "" {
			tweet.ScheduledAt = scheduledAt
		}

		// ファイル名生成
		filename := fmt.Sprintf("%s.json", time.Now().Format("2006-01-02-150405"))
		tweetsDir := "tweets"
		filepath := filepath.Join(tweetsDir, filename)

		// JSON書き出し
		data, _ := json.MarshalIndent(tweet, "", "  ")
		if err := os.MkdirAll(tweetsDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath, data, 0644); err != nil {
			return err
		}

		fmt.Printf("✅ Created: %s\n", filepath)
		return nil
	},
}

var tweetListCmd = &cobra.Command{
	Use:   "list",
	Short: "予約中のツイートを一覧表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		tweetsDir := "tweets"
		if _, err := os.Stat(tweetsDir); os.IsNotExist(err) {
			fmt.Println("予約中のツイートはありません")
			return nil
		}

		files, err := os.ReadDir(tweetsDir)
		if err != nil {
			return err
		}

		count := 0
		for _, f := range files {
			if f.IsDir() || !isJSONFile(f.Name()) {
				continue
			}
			filepath := filepath.Join(tweetsDir, f.Name())
			data, err := os.ReadFile(filepath)
			if err != nil {
				continue
			}
			var tweet Tweet
			if err := json.Unmarshal(data, &tweet); err != nil {
				continue
			}
			scheduled := "即時"
			if tweet.ScheduledAt != "" {
				scheduled = tweet.ScheduledAt
			}
			fmt.Printf("📝 %s [%s]\n   %s\n\n", f.Name(), scheduled, truncate(tweet.Text, 50))
			count++
		}

		if count == 0 {
			fmt.Println("予約中のツイートはありません")
		}
		return nil
	},
}

func isJSONFile(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == ".json"
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func init() {
	tweetAddCmd.Flags().StringP("at", "a", "", "予約日時 (例: 2026-07-15T09:00:00+09:00)")
	tweetCmd.AddCommand(tweetAddCmd)
	tweetCmd.AddCommand(tweetListCmd)
	rootCmd.AddCommand(tweetCmd)
}
