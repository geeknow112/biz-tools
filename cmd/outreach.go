package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

// OutreachEntry is one candidate outreach message draft.
type OutreachEntry struct {
	Domain        string    `json:"domain"`
	URL           string    `json:"url"`
	RiskLevel     string    `json:"risk_level"`
	RiskScore     int       `json:"risk_score"`
	FindingTitles []string  `json:"finding_titles"`
	ContactURL    string    `json:"contact_url,omitempty"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

var riskRank = map[string]int{
	"Critical": 5, "High": 4, "Medium": 3, "Low": 2, "Info": 1, "Safe": 0,
}

var (
	outreachQueueFile string
	outreachInputFile string
	outreachMinRisk   string
)

var outreachCmd = &cobra.Command{
	Use:   "outreach",
	Short: "Draft outreach messages from scan results",
	Long: `Builds a queue of candidate outreach message drafts from scan --batch
results, and looks up each site's contact page where possible.

This only drafts messages — it does not submit forms or send anything.
Sending is done manually; see "biz-tools outreach list" for the drafts.`,
}

var outreachQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Build/update the outreach draft queue from scan --batch results",
	RunE:  runOutreachQueue,
}

var outreachListCmd = &cobra.Command{
	Use:   "list",
	Short: "List outreach draft queue entries",
	RunE:  runOutreachList,
}

func init() {
	rootCmd.AddCommand(outreachCmd)
	outreachCmd.AddCommand(outreachQueueCmd, outreachListCmd)

	outreachQueueCmd.Flags().StringVarP(&outreachInputFile, "input", "i", "", "scan --batch output JSON file (required)")
	outreachQueueCmd.Flags().StringVarP(&outreachMinRisk, "min-risk", "r", "Medium", "Minimum risk to include (Low, Medium, High, Critical)")
	outreachQueueCmd.MarkFlagRequired("input")

	outreachQueueCmd.Flags().StringVarP(&outreachQueueFile, "queue", "q", "outreach_queue.json", "Outreach queue file path")
	outreachListCmd.Flags().StringVarP(&outreachQueueFile, "queue", "q", "outreach_queue.json", "Outreach queue file path")
}

// --- queue ---

func runOutreachQueue(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(outreachInputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", outreachInputFile, err)
	}
	var scanResults []*ScanResult
	if err := json.Unmarshal(data, &scanResults); err != nil {
		return fmt.Errorf("failed to parse scan results: %w", err)
	}

	queue, _ := loadOutreachQueue(outreachQueueFile)
	queuedDomains := map[string]bool{}
	for _, e := range queue {
		queuedDomains[e.Domain] = true
	}

	minRank := riskRank[outreachMinRisk]
	added := 0
	contactFound := 0

	for _, r := range scanResults {
		if riskRank[r.OverallRisk] < minRank {
			continue
		}
		domain := extractDomain(r.URL)
		if domain == "" || queuedDomains[domain] {
			continue
		}

		var titles []string
		for _, f := range r.Findings {
			titles = append(titles, f.Title)
		}

		message, err := renderOutreachMessage(config.Outreach, domain, r.URL, titles)
		if err != nil {
			return fmt.Errorf("failed to render message template: %w", err)
		}

		fmt.Printf("Looking for contact page: %s\n", domain)
		contactURL := findSiteContactPage(domain, timeout)
		if contactURL != "" {
			contactFound++
		}

		queue = append(queue, OutreachEntry{
			Domain:        domain,
			URL:           r.URL,
			RiskLevel:     r.OverallRisk,
			RiskScore:     r.RiskScore,
			FindingTitles: titles,
			ContactURL:    contactURL,
			Message:       message,
			CreatedAt:     time.Now(),
		})
		queuedDomains[domain] = true
		added++
	}

	if err := saveOutreachQueue(outreachQueueFile, queue); err != nil {
		return err
	}

	fmt.Printf("\n%d件をキューに追加（問い合わせページ判明: %d件）\n", added, contactFound)
	fmt.Printf("キュー: %s\n", outreachQueueFile)
	return nil
}

// --- list ---

func runOutreachList(cmd *cobra.Command, args []string) error {
	queue, err := loadOutreachQueue(outreachQueueFile)
	if err != nil {
		return err
	}
	if len(queue) == 0 {
		fmt.Println("キューは空です")
		return nil
	}
	for i, e := range queue {
		contact := "未検出（トップページから探してください）"
		if e.ContactURL != "" {
			contact = e.ContactURL
		}
		fmt.Printf("%d. %-8s %s (score %d) - %s\n", i+1, e.RiskLevel, e.Domain, e.RiskScore, contact)
	}
	return nil
}

// --- persistence ---

func loadOutreachQueue(path string) ([]OutreachEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var queue []OutreachEntry
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return queue, nil
}

func saveOutreachQueue(path string, queue []OutreachEntry) error {
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// --- message template ---

const defaultOutreachTemplate = `突然のご連絡失礼いたします。
Webシステム開発・保守を専門に行っております、
{{.SenderCompany}}の{{.SenderName}}と申します。

貴社のWebサイト（{{.URL}}）を公開情報の範囲で拝見した際に、
以下の点が気になりましたのでご連絡いたしました。

【確認した問題】
{{.FindingsList}}

このままですと、法人の信頼性低下や、将来的に画面が正常に表示されなくなる恐れがございます。

もし現在の管理会社様での対応が難しい状況でしたら、
弊社にて無料で原因診断をさせていただきます。

ご興味がございましたら、本メールにご返信いただくか、
下記までお気軽にご連絡ください。

{{.SenderCompany}}
{{.SenderName}}
{{.SenderEmail}}
`

type outreachTemplateData struct {
	URL           string
	FindingsList  string
	SenderName    string
	SenderCompany string
	SenderEmail   string
}

func renderOutreachMessage(cfg OutreachConfig, domain, pageURL string, findingTitles []string) (string, error) {
	tmplText := cfg.MessageTemplate
	if tmplText == "" {
		tmplText = defaultOutreachTemplate
	}
	tmpl, err := template.New("outreach").Parse(tmplText)
	if err != nil {
		return "", err
	}

	list := findingTitles
	if len(list) > 3 {
		list = list[:3]
	}
	var findingsList string
	for _, t := range list {
		findingsList += "・" + t + "\n"
	}

	data := outreachTemplateData{
		URL:           pageURL,
		FindingsList:  strings.TrimRight(findingsList, "\n"),
		SenderName:    cfg.SenderName,
		SenderCompany: cfg.SenderCompany,
		SenderEmail:   cfg.SenderEmail,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- contact page discovery (best-effort; link only, no form submission) ---

var contactLinkKeywords = []string{
	"contact", "inquiry", "toiawase", "otoiawase",
	"お問い合わせ", "お問合せ", "問い合わせ", "問合せ",
}

// findSiteContactPage fetches the homepage and returns the most likely
// contact page URL, if a matching link is found.
func findSiteContactPage(baseURL string, timeoutSec int) string {
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	homeBody, err := fetchBody(client, baseURL)
	if err != nil {
		return ""
	}
	return findContactLink(baseURL, homeBody)
}

func fetchBody(client *http.Client, rawURL string) (string, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func findContactLink(baseURL, body string) string {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			text := strings.ToLower(textContent(n))
			hrefLower := strings.ToLower(href)
			for _, kw := range contactLinkKeywords {
				kwLower := strings.ToLower(kw)
				if strings.Contains(hrefLower, kwLower) || strings.Contains(text, kwLower) {
					if resolved, err := base.Parse(href); err == nil {
						found = resolved.String()
					}
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if found != "" {
				return
			}
		}
	}
	walk(doc)
	return found
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}
