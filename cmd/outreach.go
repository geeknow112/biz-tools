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

// OutreachEntry is one candidate outreach message awaiting human review.
type OutreachEntry struct {
	Domain         string            `json:"domain"`
	URL            string            `json:"url"`
	RiskLevel      string            `json:"risk_level"`
	RiskScore      int               `json:"risk_score"`
	FindingTitles  []string          `json:"finding_titles"`
	FormURL        string            `json:"form_url,omitempty"`
	FormMethod     string            `json:"form_method,omitempty"`
	FormFields     map[string]string `json:"form_fields,omitempty"`
	ManualRequired bool              `json:"manual_required"`
	Message        string            `json:"message"`
	Status         string            `json:"status"` // pending, approved, sent, failed
	CreatedAt      time.Time         `json:"created_at"`
	SentAt         *time.Time        `json:"sent_at,omitempty"`
	Note           string            `json:"note,omitempty"`
}

var riskRank = map[string]int{
	"Critical": 5, "High": 4, "Medium": 3, "Low": 2, "Info": 1, "Safe": 0,
}

var (
	outreachQueueFile   string
	outreachInputFile   string
	outreachMinRisk     string
	outreachHistoryFile string
	outreachApproveAll  bool
	outreachDryRun      bool
)

var outreachCmd = &cobra.Command{
	Use:   "outreach",
	Short: "Review-gated outreach to sites flagged by scan",
	Long: `Builds a review queue of candidate outreach messages from batch scan
results, detecting each site's own contact form. Nothing is ever submitted
without an explicit "approve" step — this is intentionally not a fully
automatic sender.`,
}

var outreachQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Build/update the outreach queue from scan --batch results",
	RunE:  runOutreachQueue,
}

var outreachListCmd = &cobra.Command{
	Use:   "list",
	Short: "List outreach queue entries",
	RunE:  runOutreachList,
}

var outreachApproveCmd = &cobra.Command{
	Use:   "approve [domain...]",
	Short: "Approve queue entries for sending (by domain, or --all)",
	RunE:  runOutreachApprove,
}

var outreachSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Submit approved queue entries via their detected contact form",
	RunE:  runOutreachSend,
}

var outreachHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show past outreach send history",
	RunE:  runOutreachHistory,
}

func init() {
	rootCmd.AddCommand(outreachCmd)
	outreachCmd.AddCommand(outreachQueueCmd, outreachListCmd, outreachApproveCmd, outreachSendCmd, outreachHistoryCmd)

	outreachQueueCmd.Flags().StringVarP(&outreachInputFile, "input", "i", "", "scan --batch output JSON file (required)")
	outreachQueueCmd.Flags().StringVarP(&outreachMinRisk, "min-risk", "r", "Medium", "Minimum risk to include (Low, Medium, High, Critical)")
	outreachQueueCmd.MarkFlagRequired("input")

	outreachQueueCmd.Flags().StringVarP(&outreachQueueFile, "queue", "q", "outreach_queue.json", "Outreach queue file path")
	outreachListCmd.Flags().StringVarP(&outreachQueueFile, "queue", "q", "outreach_queue.json", "Outreach queue file path")
	outreachApproveCmd.Flags().StringVarP(&outreachQueueFile, "queue", "q", "outreach_queue.json", "Outreach queue file path")
	outreachSendCmd.Flags().StringVarP(&outreachQueueFile, "queue", "q", "outreach_queue.json", "Outreach queue file path")

	outreachQueueCmd.Flags().StringVar(&outreachHistoryFile, "history", "outreach_history.json", "History file, used to skip already-contacted domains")
	outreachSendCmd.Flags().StringVar(&outreachHistoryFile, "history", "outreach_history.json", "History file to record sends into")
	outreachHistoryCmd.Flags().StringVar(&outreachHistoryFile, "history", "outreach_history.json", "History file path")

	outreachApproveCmd.Flags().BoolVar(&outreachApproveAll, "all", false, "Approve every pending entry")
	outreachSendCmd.Flags().BoolVar(&outreachDryRun, "dry-run", false, "Print what would be submitted without sending")
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

	history, _ := loadOutreachHistory(outreachHistoryFile)
	contactedDomains := map[string]bool{}
	for _, h := range history {
		contactedDomains[h.Domain] = true
	}

	queue, _ := loadOutreachQueue(outreachQueueFile)
	queuedDomains := map[string]bool{}
	for _, e := range queue {
		queuedDomains[e.Domain] = true
	}

	minRank := riskRank[outreachMinRisk]
	added := 0
	formDetected := 0

	for _, r := range scanResults {
		if riskRank[r.OverallRisk] < minRank {
			continue
		}
		domain := extractDomain(r.URL)
		if domain == "" || contactedDomains[domain] || queuedDomains[domain] {
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

		fmt.Printf("Checking contact form: %s\n", domain)
		formURL, method, fields, manualRequired := detectContactForm(domain, timeout, config.Outreach.SenderName, config.Outreach.SenderEmail, message)

		entry := OutreachEntry{
			Domain:         domain,
			URL:            r.URL,
			RiskLevel:      r.OverallRisk,
			RiskScore:      r.RiskScore,
			FindingTitles:  titles,
			FormURL:        formURL,
			FormMethod:     method,
			FormFields:     fields,
			ManualRequired: manualRequired,
			Message:        message,
			Status:         "pending",
			CreatedAt:      time.Now(),
		}
		if manualRequired {
			entry.Note = "問い合わせフォームを自動検出できませんでした。手動で送信してください。"
		} else {
			formDetected++
		}

		queue = append(queue, entry)
		queuedDomains[domain] = true
		added++
	}

	if err := saveOutreachQueue(outreachQueueFile, queue); err != nil {
		return err
	}

	fmt.Printf("\n%d件をキューに追加（フォーム自動検出: %d件、手動対応: %d件）\n", added, formDetected, added-formDetected)
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
		manual := ""
		if e.ManualRequired {
			manual = " [手動対応]"
		}
		fmt.Printf("%d. [%s] %-8s %s (score %d)%s\n", i+1, e.Status, e.RiskLevel, e.Domain, e.RiskScore, manual)
	}
	return nil
}

// --- approve ---

func runOutreachApprove(cmd *cobra.Command, args []string) error {
	queue, err := loadOutreachQueue(outreachQueueFile)
	if err != nil {
		return err
	}

	targets := map[string]bool{}
	for _, a := range args {
		targets[a] = true
	}

	approved := 0
	for i := range queue {
		if queue[i].Status != "pending" {
			continue
		}
		if outreachApproveAll || targets[queue[i].Domain] {
			queue[i].Status = "approved"
			approved++
		}
	}

	if err := saveOutreachQueue(outreachQueueFile, queue); err != nil {
		return err
	}
	fmt.Printf("%d件を承認しました\n", approved)
	return nil
}

// --- send ---

func runOutreachSend(cmd *cobra.Command, args []string) error {
	queue, err := loadOutreachQueue(outreachQueueFile)
	if err != nil {
		return err
	}
	history, _ := loadOutreachHistory(outreachHistoryFile)

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	sent := 0

	for i := range queue {
		if queue[i].Status != "approved" {
			continue
		}
		if queue[i].ManualRequired || queue[i].FormURL == "" {
			fmt.Printf("スキップ（手動対応が必要）: %s\n", queue[i].Domain)
			continue
		}

		if outreachDryRun {
			fmt.Printf("[dry-run] POST %s (%s) fields=%v\n", queue[i].FormURL, queue[i].FormMethod, queue[i].FormFields)
			continue
		}

		fmt.Printf("送信中: %s ... ", queue[i].Domain)
		if err := submitContactForm(client, queue[i].FormURL, queue[i].FormMethod, queue[i].FormFields); err != nil {
			fmt.Printf("失敗: %v\n", err)
			queue[i].Status = "failed"
			queue[i].Note = err.Error()
			continue
		}
		fmt.Println("完了")

		now := time.Now()
		queue[i].Status = "sent"
		queue[i].SentAt = &now
		history = append(history, OutreachHistoryEntry{Domain: queue[i].Domain, URL: queue[i].URL, SentAt: now})
		sent++
	}

	if !outreachDryRun {
		if err := saveOutreachQueue(outreachQueueFile, queue); err != nil {
			return err
		}
		if err := saveOutreachHistory(outreachHistoryFile, history); err != nil {
			return err
		}
	}

	fmt.Printf("\n%d件送信しました\n", sent)
	return nil
}

// --- history ---

type OutreachHistoryEntry struct {
	Domain string    `json:"domain"`
	URL    string    `json:"url"`
	SentAt time.Time `json:"sent_at"`
}

func runOutreachHistory(cmd *cobra.Command, args []string) error {
	history, err := loadOutreachHistory(outreachHistoryFile)
	if err != nil {
		return err
	}
	if len(history) == 0 {
		fmt.Println("送信履歴はありません")
		return nil
	}
	for _, h := range history {
		fmt.Printf("%s  %s\n", h.SentAt.Format("2006-01-02 15:04"), h.Domain)
	}
	return nil
}

// --- persistence helpers ---

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

func loadOutreachHistory(path string) ([]OutreachHistoryEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var history []OutreachHistoryEntry
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return history, nil
}

func saveOutreachHistory(path string, history []OutreachHistoryEntry) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// --- message template ---

const defaultOutreachTemplate = `初めてご連絡失礼いたします。
{{.SenderCompany}}の{{.SenderName}}と申します。

貴社のWebサイト（{{.URL}}）を公開情報の範囲で拝見した際に、以下の点が気になりましたのでご連絡いたしました。

{{.FindingsList}}

もしよろしければ、無料の簡易診断レポートをお送りいたします。ご興味があればご返信いただけますと幸いです。
不要でしたら本メッセージは読み流していただいて構いません。

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

// --- contact form detection (best-effort heuristics) ---

var contactLinkKeywords = []string{
	"contact", "inquiry", "toiawase", "otoiawase",
	"お問い合わせ", "お問合せ", "問い合わせ", "問合せ",
}

var nameFieldKeywords = []string{"name", "namae", "氏名", "お名前", "your-name"}
var emailFieldKeywords = []string{"email", "mail", "メール", "your-email"}
var messageFieldKeywords = []string{
	"message", "content", "inquiry", "detail", "comment", "body",
	"お問い合わせ", "お問合せ", "ご相談内容", "内容", "your-message",
}

// detectContactForm fetches the homepage, follows the most likely contact
// link, and tries to identify a submittable inquiry form on that page.
// manualRequired=true means no form could be confidently identified.
//
// TODO(security): baseURL comes from crawl/scan input with no restriction on
// resolving to private/link-local addresses (e.g. 169.254.169.254). Low risk
// today since input is sourced from real search results, but worth adding an
// IP allowlist/denylist check before fetching if this is ever fed
// less-trusted input.
func detectContactForm(baseURL string, timeoutSec int, senderName, senderEmail, message string) (formURL, method string, fields map[string]string, manualRequired bool) {
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}

	homeBody, err := fetchBody(client, baseURL)
	if err != nil {
		return "", "", nil, true
	}

	pageURL := baseURL
	pageBody := homeBody
	if link := findContactLink(baseURL, homeBody); link != "" && link != baseURL {
		if body, err := fetchBody(client, link); err == nil {
			pageURL = link
			pageBody = body
		}
	}

	action, m, f, ok := parseContactForm(pageURL, pageBody, senderName, senderEmail, message)
	if !ok {
		return "", "", nil, true
	}
	return action, m, f, false
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

// parseContactForm looks for a <form> containing at least an email-like and
// message-like field. Hidden fields (CSRF tokens etc.) are preserved as-is.
func parseContactForm(pageURL, body, senderName, senderEmail, message string) (action, method string, fields map[string]string, ok bool) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return "", "", nil, false
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", "", nil, false
	}

	var forms []*html.Node
	var collectForms func(*html.Node)
	collectForms = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" {
			forms = append(forms, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectForms(c)
		}
	}
	collectForms(doc)

	for _, form := range forms {
		f := map[string]string{}
		hasEmail, hasMessage := false, false

		var collectInputs func(*html.Node)
		collectInputs = func(n *html.Node) {
			if n.Type == html.ElementNode && (n.Data == "input" || n.Data == "textarea") {
				name := attr(n, "name")
				if name != "" {
					typ := strings.ToLower(attr(n, "type"))
					if typ == "submit" || typ == "button" || typ == "image" {
						return
					}
					identifier := strings.ToLower(name + " " + attr(n, "id") + " " + attr(n, "placeholder"))
					switch {
					case typ == "hidden":
						f[name] = attr(n, "value")
					case matchesAny(identifier, emailFieldKeywords):
						f[name] = senderEmail
						hasEmail = true
					case matchesAny(identifier, messageFieldKeywords):
						f[name] = message
						hasMessage = true
					case matchesAny(identifier, nameFieldKeywords):
						f[name] = senderName
					default:
						f[name] = attr(n, "value")
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				collectInputs(c)
			}
		}
		collectInputs(form)

		if hasEmail && hasMessage {
			actionAttr := attr(form, "action")
			resolvedAction := pageURL
			if actionAttr != "" {
				if resolved, err := base.Parse(actionAttr); err == nil {
					resolvedAction = resolved.String()
				}
			}
			m := strings.ToUpper(attr(form, "method"))
			if m == "" {
				m = "POST"
			}
			return resolvedAction, m, f, true
		}
	}

	return "", "", nil, false
}

func matchesAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, strings.ToLower(kw)) {
			return true
		}
	}
	return false
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

// TODO(reliability): hidden fields (CSRF tokens etc.) are captured at `queue`
// time but only submitted whenever a human later runs `send` — by then the
// token has often expired or was tied to a since-closed session, so the
// submission may be rejected server-side. This is an inherent tension with
// the review-gated design (no fully-automatic immediate send); fixing it
// properly would mean re-fetching the form right before submit instead of
// reusing the value captured at queue time.
func submitContactForm(client *http.Client, formURL, method string, fields map[string]string) error {
	values := url.Values{}
	for k, v := range fields {
		values.Set(k, v)
	}

	var resp *http.Response
	var err error
	if method == "GET" {
		resp, err = client.Get(formURL + "?" + values.Encode())
	} else {
		resp, err = client.PostForm(formURL, values)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("form returned HTTP %d", resp.StatusCode)
	}
	return nil
}
