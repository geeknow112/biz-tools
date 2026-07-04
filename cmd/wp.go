package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

type wpPage struct {
	ID int `json:"id"`
}

// Raw HTML passthrough is required because page content (e.g. layout tables)
// is authored as HTML inside the Markdown source. The source is trusted
// (only ever committed via this repo's own PR flow), so this is safe.
var markdownConverter = goldmark.New(
	goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
)

func markdownToHTML(markdown []byte) (string, error) {
	var buf bytes.Buffer
	if err := markdownConverter.Convert(markdown, &buf); err != nil {
		return "", fmt.Errorf("failed to convert markdown: %w", err)
	}
	return buf.String(), nil
}

func wpFindPageBySlug(cfg PlatformConfig, slug string) (int, error) {
	reqURL := fmt.Sprintf("%s/wp-json/wp/v2/pages?slug=%s&status=any", cfg.URL, slug)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(cfg.Username, cfg.AppPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to reach WordPress: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("WordPress returned %d: %s", resp.StatusCode, string(body))
	}

	var pages []wpPage
	if err := json.Unmarshal(body, &pages); err != nil {
		return 0, fmt.Errorf("failed to parse WordPress response: %w", err)
	}
	if len(pages) == 0 {
		return 0, fmt.Errorf("no WordPress page found with slug %q", slug)
	}
	return pages[0].ID, nil
}

func wpUpdatePageContent(cfg PlatformConfig, pageID int, html string) error {
	payload, err := json.Marshal(map[string]string{"content": html})
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/wp-json/wp/v2/pages/%d", cfg.URL, pageID)
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.Username, cfg.AppPassword)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach WordPress: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("WordPress returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// wpPublishPage converts the given markdown file to HTML and pushes it to the
// WordPress page whose slug matches the file's base name (without extension).
func wpPublishPage(cfg PlatformConfig, slug string, markdown []byte) error {
	if cfg.URL == "" || cfg.Username == "" || cfg.AppPassword == "" {
		return fmt.Errorf("wordpress platform is missing url/username/app_password in config.yaml")
	}

	html, err := markdownToHTML(markdown)
	if err != nil {
		return err
	}

	pageID, err := wpFindPageBySlug(cfg, slug)
	if err != nil {
		return err
	}

	return wpUpdatePageContent(cfg, pageID, html)
}

// publishToWordPress pulls the just-merged commit, reads the page's markdown
// from the repo working copy, and pushes it to the matching WordPress page.
func publishToWordPress(cfg PlatformConfig, file string) error {
	currentBranch, err := gitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	if _, err := gitCommand("pull", "origin", strings.TrimSpace(currentBranch)); err != nil {
		return fmt.Errorf("failed to pull merged changes: %w", err)
	}

	destPath := platformDestPath("wordpress", file)
	content, err := os.ReadFile(destPath)
	if err != nil {
		return fmt.Errorf("failed to read merged file %s: %w", destPath, err)
	}

	slug := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	return wpPublishPage(cfg, slug, content)
}
