package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// CrawlResult is one deduplicated candidate site found via Google dork search.
type CrawlResult struct {
	Domain  string `json:"domain"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Query   string `json:"query"`
}

type googleSearchResponse struct {
	Items []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"items"`
}

var (
	crawlQueryFile string
	crawlOutput    string
	crawlMaxPages  int
)

var crawlCmd = &cobra.Command{
	Use:   "crawl",
	Short: "Discover candidate sites via Google Custom Search dork queries",
	Long: `Runs one or more Google dork-style search queries (site:, inurl:, "..." exclusions)
against the Google Custom Search JSON API to discover publicly indexed pages
exposing PHP errors/warnings or other signs of neglect, then writes a
deduplicated (by domain) candidate list.

Requires crawl.google_api_key and crawl.google_cse_id in config.yaml.
Uses the official Custom Search API only — it does not scrape google.com
search result pages directly.

Example:
  biz-tools crawl -q queries.txt -o candidates.json`,
	RunE: runCrawl,
}

func init() {
	rootCmd.AddCommand(crawlCmd)
	crawlCmd.Flags().StringVarP(&crawlQueryFile, "queries", "q", "", "Path to a file with one dork query per line (required)")
	crawlCmd.Flags().StringVarP(&crawlOutput, "output", "o", "candidates.json", "Output file path (.json or .csv)")
	crawlCmd.Flags().IntVarP(&crawlMaxPages, "max-pages", "p", 3, "Result pages to fetch per query (10 results/page)")
	crawlCmd.MarkFlagRequired("queries")
}

func runCrawl(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if config.Crawl.GoogleAPIKey == "" || config.Crawl.GoogleCSEID == "" {
		return fmt.Errorf("crawl.google_api_key / crawl.google_cse_id not set in config.yaml")
	}

	lines, err := readLines(crawlQueryFile)
	if err != nil {
		return fmt.Errorf("failed to read queries file: %w", err)
	}

	seenDomains := map[string]bool{}
	var results []CrawlResult
	client := &http.Client{Timeout: 15 * time.Second}

	for _, query := range lines {
		query = strings.TrimSpace(query)
		if query == "" || strings.HasPrefix(query, "#") {
			continue
		}
		fmt.Printf("Searching: %s\n", query)

		for page := 0; page < crawlMaxPages; page++ {
			start := page*10 + 1
			if start > 91 {
				break // Google CSE API caps startIndex at 91 (100 results max)
			}
			resp, err := googleCustomSearch(client, config.Crawl.GoogleAPIKey, config.Crawl.GoogleCSEID, query, start)
			if err != nil {
				fmt.Printf("  page %d: %v\n", page+1, err)
				break
			}
			if len(resp.Items) == 0 {
				break
			}
			for _, item := range resp.Items {
				domain := extractDomain(item.Link)
				if domain == "" || seenDomains[domain] {
					continue
				}
				seenDomains[domain] = true
				results = append(results, CrawlResult{
					Domain:  domain,
					URL:     item.Link,
					Title:   item.Title,
					Snippet: item.Snippet,
					Query:   query,
				})
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	fmt.Printf("\nFound %d unique candidate sites\n", len(results))
	return writeCrawlResults(results, crawlOutput)
}

func googleCustomSearch(client *http.Client, apiKey, cseID, query string, start int) (*googleSearchResponse, error) {
	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("cx", cseID)
	params.Set("q", query)
	params.Set("start", fmt.Sprintf("%d", start))

	resp, err := client.Get("https://www.googleapis.com/customsearch/v1?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API returned %d: %s", resp.StatusCode, string(body))
	}

	var result googleSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func writeCrawlResults(results []CrawlResult, outputPath string) error {
	if strings.HasSuffix(outputPath, ".csv") {
		f, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer f.Close()

		w := csv.NewWriter(f)
		defer w.Flush()
		w.Write([]string{"domain", "url", "title", "snippet", "query"})
		for _, r := range results {
			w.Write([]string{r.Domain, r.URL, r.Title, r.Snippet, r.Query})
		}
		fmt.Printf("Saved to %s\n", outputPath)
		return nil
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return err
	}
	fmt.Printf("Saved to %s\n", outputPath)
	return nil
}
