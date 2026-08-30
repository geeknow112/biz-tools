package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// X (Twitter) posting is immediate/public the moment it succeeds, unlike
// zenn/qiita/wordpress which go through a reviewable draft PR first (see
// media.go). That's why it lives under its own "media post" subcommand
// instead of "media draft"/"media publish".

const (
	xTweetsEndpoint = "https://api.x.com/2/tweets"
	xMaxPostLength  = 280
)

var xURLPattern = regexp.MustCompile(`https?://\S+`)

var mediaPostCmd = &cobra.Command{
	Use:   "post [file]",
	Short: "Immediately post content (no PR review — currently: X)",
	Long: `Post content immediately to a platform that has no draft/review
concept, unlike "media draft"/"media publish" which go through a GitHub PR.

Currently only the "x" platform is supported.

Example:
  biz-tools media post tweet.md -p x --dry-run
  biz-tools media post tweet.md -p x
  biz-tools media post thread.md -p x --thread`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, _ := cmd.Flags().GetString("platform")
		thread, _ := cmd.Flags().GetBool("thread")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		file := args[0]

		switch platform {
		case "x":
			return runPostX(file, thread, dryRun)
		default:
			return fmt.Errorf("media post does not support platform '%s' yet (supported: x)", platform)
		}
	},
}

func init() {
	mediaCmd.AddCommand(mediaPostCmd)
	mediaPostCmd.Flags().StringP("platform", "p", "x", "Target platform (currently only: x)")
	mediaPostCmd.Flags().Bool("thread", false, "Split content over 280 chars into a thread instead of failing")
	mediaPostCmd.Flags().Bool("dry-run", false, "Show what would be posted without actually posting")
}

func runPostX(file string, thread, dryRun bool) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	xConfig, ok := config.Platforms["x"]
	if !ok {
		return fmt.Errorf("platform 'x' not configured in config.yaml (see config.yaml.example)")
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return fmt.Errorf("file is empty: %s", file)
	}

	chunks, err := splitForX(text, thread)
	if err != nil {
		return err
	}

	fmt.Printf("File: %s\n", file)
	fmt.Printf("Posts: %d\n", len(chunks))
	for i, c := range chunks {
		fmt.Printf("\n--- Post %d/%d (%d chars) ---\n%s\n", i+1, len(chunks), xWeightedLength(c), c)
	}

	if dryRun {
		fmt.Println("\n[dry-run] Nothing was posted. Remove --dry-run to post for real.")
		return nil
	}

	if xConfig.APIKey == "" || xConfig.APISecret == "" || xConfig.AccessToken == "" || xConfig.AccessTokenSecret == "" {
		return fmt.Errorf("platform 'x' is missing api_key/api_secret/access_token/access_token_secret in config.yaml")
	}

	var previousID string
	fmt.Println()
	for i, c := range chunks {
		id, err := postTweet(xConfig, c, previousID)
		if err != nil {
			if i > 0 {
				return fmt.Errorf("posted %d/%d before failing on post %d: %w", i, len(chunks), i+1, err)
			}
			return fmt.Errorf("failed to post: %w", err)
		}
		fmt.Printf("Posted %d/%d: https://x.com/i/web/status/%s\n", i+1, len(chunks), id)
		previousID = id
	}

	return nil
}

// splitForX validates the post length and, if it's over the limit and
// thread is true, splits it into a sequence of <=280-char chunks suitable
// for posting as a reply chain (a "thread").
func splitForX(text string, thread bool) ([]string, error) {
	total := xWeightedLength(text)
	if total <= xMaxPostLength {
		return []string{text}, nil
	}
	if !thread {
		return nil, fmt.Errorf("content is %d chars, over the %d limit; rerun with --thread to split it into a thread, or shorten it", total, xMaxPostLength)
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, fmt.Errorf("no content to post")
	}

	// Reserve room for the " (N/M)" suffix appended to every chunk below.
	// 8 chars covers up to "(10/10)"; threads longer than that are
	// vanishingly rare for this tool's use case.
	const suffixReserve = 8
	limit := xMaxPostLength - suffixReserve

	var chunks []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}

	for _, w := range words {
		// A single word longer than the limit: hard-cut it by rune so it
		// doesn't get stuck as an oversized chunk.
		if xWeightedLength(w) > limit {
			flush()
			for _, part := range hardSplit(w, limit) {
				chunks = append(chunks, part)
			}
			continue
		}

		candidate := w
		if current.Len() > 0 {
			candidate = current.String() + " " + w
		}
		if xWeightedLength(candidate) > limit {
			flush()
			current.WriteString(w)
		} else {
			if current.Len() > 0 {
				current.WriteString(" ")
			}
			current.WriteString(w)
		}
	}
	flush()

	n := len(chunks)
	for i := range chunks {
		chunks[i] = fmt.Sprintf("%s (%d/%d)", chunks[i], i+1, n)
	}
	return chunks, nil
}

func hardSplit(s string, limit int) []string {
	var parts []string
	runes := []rune(s)
	var current []rune
	length := 0
	for _, r := range runes {
		w := 1
		if isWideRune(r) {
			w = 2
		}
		if length+w > limit && len(current) > 0 {
			parts = append(parts, string(current))
			current = nil
			length = 0
		}
		current = append(current, r)
		length += w
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

// xWeightedLength approximates X's character-counting rules for the 280
// char limit: URLs are collapsed to a fixed t.co-length placeholder (23
// chars, X's current shortened-link length), and CJK/wide characters count
// as 2 instead of 1, matching how they're weighted on X. This is a
// simplified approximation of the official twitter-text weighted-length
// algorithm (which has more unicode range edge cases) — it's accurate for
// the common case this tool handles: Japanese text with plain https URLs.
func xWeightedLength(s string) int {
	replaced := xURLPattern.ReplaceAllStringFunc(s, func(string) string {
		return strings.Repeat("x", 23)
	})
	length := 0
	for _, r := range replaced {
		if isWideRune(r) {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func isWideRune(r rune) bool {
	switch {
	case r >= 0x3040 && r <= 0x30FF: // Hiragana, Katakana
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Unified Ideographs Extension A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return true
	case r >= 0xFF00 && r <= 0xFFEF: // Fullwidth/Halfwidth Forms
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul syllables
		return true
	}
	return false
}

type xTweetRequest struct {
	Text  string         `json:"text"`
	Reply *xReplyRequest `json:"reply,omitempty"`
}

type xReplyRequest struct {
	InReplyToTweetID string `json:"in_reply_to_tweet_id"`
}

type xTweetResponse struct {
	Data struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"data"`
	Errors []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

// postTweet posts a single tweet via X API v2 (POST /2/tweets), authenticated
// with OAuth 1.0a User Context. If replyToID is non-empty, the tweet is
// posted as a reply to it (used to chain thread posts together).
func postTweet(cfg PlatformConfig, text, replyToID string) (string, error) {
	reqBody := xTweetRequest{Text: text}
	if replyToID != "" {
		reqBody.Reply = &xReplyRequest{InReplyToTweetID: replyToID}
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// JSON bodies are not form-encoded, so no extra params are signed here
	// besides the standard oauth_* ones (see oauth1.go).
	authHeader, err := buildOAuth1Header(http.MethodPost, xTweetsEndpoint, nil,
		cfg.APIKey, cfg.APISecret, cfg.AccessToken, cfg.AccessTokenSecret)
	if err != nil {
		return "", fmt.Errorf("failed to build OAuth1 header: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, xTweetsEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach X API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("X API returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed xTweetResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse X API response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return "", fmt.Errorf("X API error: %s - %s", parsed.Errors[0].Title, parsed.Errors[0].Detail)
	}
	if parsed.Data.ID == "" {
		return "", fmt.Errorf("X API response missing tweet id: %s", string(body))
	}
	return parsed.Data.ID, nil
}
