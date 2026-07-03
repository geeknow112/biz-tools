package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Severity levels for findings
const (
	SeverityCritical = "Critical"
	SeverityHigh     = "High"
	SeverityMedium   = "Medium"
	SeverityLow      = "Low"
	SeverityInfo     = "Info"
)

// ScanResult holds all scan results
type ScanResult struct {
	URL           string
	ScanTime      time.Time
	SSL           SSLResult
	Headers       HeadersResult
	CMS           CMSResult
	ServerInfo    ServerInfoResult
	DNS           DNSResult
	OverallRisk   string
	RiskScore     int
	Findings      []Finding
}

type SSLResult struct {
	Enabled       bool
	ValidFrom     time.Time
	ValidUntil    time.Time
	DaysRemaining int
	Issuer        string
	Protocol      string
	Risk          string
}

type HeadersResult struct {
	HSTS              bool
	XFrameOptions     string
	XContentType      bool
	CSP               bool
	XSSProtection     bool
	MissingHeaders    []string
	Risk              string
}

type CMSResult struct {
	Detected      bool
	Name          string
	Version       string
	VersionExposed bool
	Risk          string
}

type ServerInfoResult struct {
	Server        string
	XPoweredBy    string
	Exposed       bool
	Risk          string
}

type DNSResult struct {
	HasMX         bool
	HasSPF        bool
	HasDMARC      bool
	MXRecords     []string
	Risk          string
}

type Finding struct {
	Severity    string // Critical, High, Medium, Low, Info
	Category    string
	Title       string
	Description string
	Remediation string
}

var (
	outputFormat string
	outputFile   string
	timeout      int
)

var scanCmd = &cobra.Command{
	Use:   "scan [url]",
	Short: "Security scan for websites (public information only)",
	Long: `Performs a lightweight security scan using only publicly available information.

This tool does NOT perform intrusive scanning. It only checks:
- SSL certificate validity and configuration
- HTTP security headers
- CMS detection (WordPress, etc.)
- Server information exposure
- DNS security settings (SPF, DMARC)

Example:
  biz-tools scan https://example.com
  biz-tools scan https://example.com -o markdown -f report.md`,
	Args: cobra.ExactArgs(1),
	Run:  runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, markdown, json, html)")
	scanCmd.Flags().StringVarP(&outputFile, "file", "f", "", "Output file path")
	scanCmd.Flags().IntVarP(&timeout, "timeout", "t", 10, "Request timeout in seconds")
}

func runScan(cmd *cobra.Command, args []string) {
	url := normalizeURL(args[0])
	
	fmt.Printf("🔍 Scanning: %s\n", url)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	result := &ScanResult{
		URL:      url,
		ScanTime: time.Now(),
		Findings: []Finding{},
	}
	
	// Run all checks
	fmt.Print("Checking SSL certificate... ")
	result.SSL = checkSSL(url)
	fmt.Println("✓")
	
	fmt.Print("Checking HTTP headers... ")
	result.Headers, result.ServerInfo = checkHeaders(url)
	fmt.Println("✓")
	
	fmt.Print("Detecting CMS... ")
	result.CMS = detectCMS(url)
	fmt.Println("✓")
	
	fmt.Print("Checking DNS records... ")
	result.DNS = checkDNS(url)
	fmt.Println("✓")
	
	// Aggregate findings
	aggregateFindings(result)
	
	// Calculate overall risk
	calculateOverallRisk(result)
	
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Output results
	output := formatOutput(result, outputFormat)
	
	if outputFile != "" {
		err := os.WriteFile(outputFile, []byte(output), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📄 Report saved to: %s\n", outputFile)
	} else {
		fmt.Println(output)
	}
}

func normalizeURL(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}

func getHost(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	parts := strings.Split(url, "/")
	return parts[0]
}

func checkSSL(url string) SSLResult {
	result := SSLResult{Risk: SeverityInfo}
	
	if strings.HasPrefix(url, "http://") {
		result.Enabled = false
		result.Risk = SeverityCritical
		return result
	}
	
	host := getHost(url)
	
	// Use context and net.Dialer for proper timeout control
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", host+":443")
	if err != nil {
		result.Enabled = false
		result.Risk = SeverityCritical
		return result
	}
	
	conn := tls.Client(netConn, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	})
	conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	
	err = conn.Handshake()
	if err != nil {
		conn.Close()
		// Retry with InsecureSkipVerify to get cert info even if invalid
		netConn, err = dialer.DialContext(ctx, "tcp", host+":443")
		if err != nil {
			result.Enabled = false
			result.Risk = SeverityCritical
			return result
		}
		conn = tls.Client(netConn, &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: true,
		})
		conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
		if err = conn.Handshake(); err != nil {
			conn.Close()
			result.Enabled = false
			result.Risk = SeverityCritical
			return result
		}
		result.Risk = SeverityHigh
	}
	defer conn.Close()
	
	result.Enabled = true
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) > 0 {
		cert := certs[0]
		result.ValidFrom = cert.NotBefore
		result.ValidUntil = cert.NotAfter
		result.DaysRemaining = int(time.Until(cert.NotAfter).Hours() / 24)
		result.Issuer = cert.Issuer.CommonName
		
		switch conn.ConnectionState().Version {
		case tls.VersionTLS13:
			result.Protocol = "TLS 1.3"
		case tls.VersionTLS12:
			result.Protocol = "TLS 1.2"
		case tls.VersionTLS11:
			result.Protocol = "TLS 1.1"
			result.Risk = SeverityMedium
		case tls.VersionTLS10:
			result.Protocol = "TLS 1.0"
			result.Risk = SeverityHigh
		}
		
		if result.DaysRemaining < 0 {
			result.Risk = SeverityCritical
		} else if result.DaysRemaining < 14 {
			result.Risk = SeverityHigh
		} else if result.DaysRemaining < 30 {
			result.Risk = SeverityMedium
		}
	}
	
	return result
}

func checkHeaders(url string) (HeadersResult, ServerInfoResult) {
	headers := HeadersResult{Risk: SeverityInfo}
	server := ServerInfoResult{Risk: SeverityInfo}
	
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	
	resp, err := client.Get(url)
	if err != nil {
		headers.Risk = "Unknown"
		return headers, server
	}
	defer resp.Body.Close()
	
	// Check security headers
	if resp.Header.Get("Strict-Transport-Security") != "" {
		headers.HSTS = true
	} else {
		headers.MissingHeaders = append(headers.MissingHeaders, "Strict-Transport-Security")
	}
	
	headers.XFrameOptions = resp.Header.Get("X-Frame-Options")
	if headers.XFrameOptions == "" {
		headers.MissingHeaders = append(headers.MissingHeaders, "X-Frame-Options")
	}
	
	if resp.Header.Get("X-Content-Type-Options") == "nosniff" {
		headers.XContentType = true
	} else {
		headers.MissingHeaders = append(headers.MissingHeaders, "X-Content-Type-Options")
	}
	
	if resp.Header.Get("Content-Security-Policy") != "" {
		headers.CSP = true
	} else {
		headers.MissingHeaders = append(headers.MissingHeaders, "Content-Security-Policy")
	}
	
	if resp.Header.Get("X-XSS-Protection") != "" {
		headers.XSSProtection = true
	}
	
	// Calculate headers risk
	missingCount := len(headers.MissingHeaders)
	if missingCount >= 4 {
		headers.Risk = SeverityHigh
	} else if missingCount >= 2 {
		headers.Risk = SeverityMedium
	} else if missingCount >= 1 {
		headers.Risk = SeverityLow
	}
	
	// Check server info exposure
	server.Server = resp.Header.Get("Server")
	server.XPoweredBy = resp.Header.Get("X-Powered-By")
	
	if server.Server != "" || server.XPoweredBy != "" {
		server.Exposed = true
		server.Risk = SeverityLow
		if strings.Contains(server.Server, "/") || strings.Contains(server.XPoweredBy, "/") {
			server.Risk = SeverityMedium
		}
	}
	
	return headers, server
}

// detectCMS performs simple CMS detection based on common patterns.
// NOTE: This detection is heuristic-based and not definitive.
// CMS operators can customize paths and remove signatures,
// so absence of detection does not guarantee no CMS is present.
func detectCMS(url string) CMSResult {
	result := CMSResult{Risk: SeverityInfo}
	
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*100)) // Read max 100KB
	if err != nil {
		return result
	}
	bodyStr := string(body)
	
	// WordPress detection (heuristic - not 100% reliable)
	wpPatterns := []string{
		`wp-content`,
		`wp-includes`,
		`wp-json`,
		`wordpress`,
	}
	for _, pattern := range wpPatterns {
		if strings.Contains(strings.ToLower(bodyStr), pattern) {
			result.Detected = true
			result.Name = "WordPress"
			break
		}
	}
	
	// Check WordPress version exposure
	if result.Name == "WordPress" {
		versionRegex := regexp.MustCompile(`content="WordPress\s+([\d.]+)"`)
		matches := versionRegex.FindStringSubmatch(bodyStr)
		if len(matches) > 1 {
			result.Version = matches[1]
			result.VersionExposed = true
			result.Risk = SeverityMedium
		}
		
		// Check generator meta tag
		genRegex := regexp.MustCompile(`<meta name="generator" content="WordPress\s*([\d.]*)"`)
		genMatches := genRegex.FindStringSubmatch(bodyStr)
		if len(genMatches) > 1 && genMatches[1] != "" {
			result.Version = genMatches[1]
			result.VersionExposed = true
			result.Risk = SeverityMedium
		}
	}
	
	// Drupal detection
	drupalPatterns := []string{
		`drupal`,
		`sites/default`,
		`/core/misc/drupal.js`,
	}
	if !result.Detected {
		for _, pattern := range drupalPatterns {
			if strings.Contains(strings.ToLower(bodyStr), pattern) {
				result.Detected = true
				result.Name = "Drupal"
				break
			}
		}
	}
	
	// Joomla detection
	if !result.Detected && strings.Contains(strings.ToLower(bodyStr), "joomla") {
		result.Detected = true
		result.Name = "Joomla"
	}
	
	return result
}

func checkDNS(url string) DNSResult {
	result := DNSResult{Risk: SeverityInfo}
	host := getHost(url)
	
	// Get domain (remove subdomain for DNS checks)
	parts := strings.Split(host, ".")
	domain := host
	if len(parts) > 2 {
		domain = strings.Join(parts[len(parts)-2:], ".")
	}
	
	// Check MX records
	mx, err := net.LookupMX(domain)
	if err == nil && len(mx) > 0 {
		result.HasMX = true
		for _, m := range mx {
			result.MXRecords = append(result.MXRecords, m.Host)
		}
	}
	
	// Check SPF (TXT record)
	txt, err := net.LookupTXT(domain)
	if err == nil {
		for _, t := range txt {
			if strings.HasPrefix(t, "v=spf1") {
				result.HasSPF = true
				break
			}
		}
	}
	
	// Check DMARC
	dmarc, err := net.LookupTXT("_dmarc." + domain)
	if err == nil {
		for _, d := range dmarc {
			if strings.HasPrefix(d, "v=DMARC1") {
				result.HasDMARC = true
				break
			}
		}
	}
	
	// Calculate DNS risk
	if result.HasMX && !result.HasSPF && !result.HasDMARC {
		result.Risk = SeverityHigh
	} else if result.HasMX && (!result.HasSPF || !result.HasDMARC) {
		result.Risk = SeverityMedium
	}
	
	return result
}

func aggregateFindings(r *ScanResult) {
	// SSL findings
	if !r.SSL.Enabled {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityCritical,
			Category:    "SSL/TLS",
			Title:       "SSL証明書なし",
			Description: "サイトがHTTPSで保護されていません",
			Remediation: "SSL証明書を導入してください（Let's Encrypt推奨）",
		})
	} else {
		if r.SSL.DaysRemaining < 0 {
			r.Findings = append(r.Findings, Finding{
				Severity:    SeverityCritical,
				Category:    "SSL/TLS",
				Title:       "SSL証明書の期限切れ",
				Description: fmt.Sprintf("証明書は%d日前に期限切れしています", -r.SSL.DaysRemaining),
				Remediation: "直ちに証明書を更新してください",
			})
		} else if r.SSL.DaysRemaining < 14 {
			r.Findings = append(r.Findings, Finding{
				Severity:    SeverityHigh,
				Category:    "SSL/TLS",
				Title:       "SSL証明書の期限が近い",
				Description: fmt.Sprintf("証明書はあと%d日で期限切れします", r.SSL.DaysRemaining),
				Remediation: "証明書の更新を準備してください",
			})
		}
		if r.SSL.Protocol == "TLS 1.0" || r.SSL.Protocol == "TLS 1.1" {
			r.Findings = append(r.Findings, Finding{
				Severity:    SeverityMedium,
				Category:    "SSL/TLS",
				Title:       "古いTLSバージョン",
				Description: fmt.Sprintf("%sが使用されています", r.SSL.Protocol),
				Remediation: "TLS 1.2以上にアップグレードしてください",
			})
		}
	}
	
	// Header findings
	if !r.Headers.HSTS {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityMedium,
			Category:    "HTTPヘッダー",
			Title:       "HSTSヘッダーなし",
			Description: "Strict-Transport-Securityヘッダーが設定されていません",
			Remediation: "HSTSヘッダーを追加してHTTPS接続を強制してください",
		})
	}
	if r.Headers.XFrameOptions == "" {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityMedium,
			Category:    "HTTPヘッダー",
			Title:       "X-Frame-Optionsなし",
			Description: "クリックジャッキング攻撃に対して脆弱です",
			Remediation: "X-Frame-Options: DENY または SAMEORIGIN を設定してください",
		})
	}
	if !r.Headers.CSP {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityLow,
			Category:    "HTTPヘッダー",
			Title:       "Content-Security-Policyなし",
			Description: "CSPヘッダーが設定されていません",
			Remediation: "CSPを設定してXSS攻撃を軽減してください",
		})
	}
	
	// CMS findings
	if r.CMS.VersionExposed {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityMedium,
			Category:    "CMS",
			Title:       fmt.Sprintf("%sバージョン露出", r.CMS.Name),
			Description: fmt.Sprintf("%s %sが検出されました", r.CMS.Name, r.CMS.Version),
			Remediation: "バージョン情報を非表示にしてください（generatorメタタグの削除）",
		})
	}
	
	// Server info findings
	if r.ServerInfo.Exposed {
		severity := SeverityLow
		if r.ServerInfo.Risk == SeverityMedium {
			severity = SeverityMedium
		}
		r.Findings = append(r.Findings, Finding{
			Severity:    severity,
			Category:    "サーバー情報",
			Title:       "サーバー情報の露出",
			Description: fmt.Sprintf("Server: %s, X-Powered-By: %s", r.ServerInfo.Server, r.ServerInfo.XPoweredBy),
			Remediation: "サーバーヘッダーを非表示にしてください",
		})
	}
	
	// DNS findings
	if r.DNS.HasMX && !r.DNS.HasSPF {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityHigh,
			Category:    "メールセキュリティ",
			Title:       "SPFレコードなし",
			Description: "メール送信元の認証がされていません",
			Remediation: "SPFレコードを設定してなりすまし対策をしてください",
		})
	}
	if r.DNS.HasMX && !r.DNS.HasDMARC {
		r.Findings = append(r.Findings, Finding{
			Severity:    SeverityMedium,
			Category:    "メールセキュリティ",
			Title:       "DMARCレコードなし",
			Description: "メールのなりすまし検証ポリシーがありません",
			Remediation: "DMARCレコードを設定してください",
		})
	}
}

func calculateOverallRisk(r *ScanResult) {
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityHigh:
			highCount++
		case SeverityMedium:
			mediumCount++
		}
	}
	
	r.RiskScore = criticalCount*10 + highCount*5 + mediumCount*2
	
	if criticalCount > 0 {
		r.OverallRisk = SeverityCritical
	} else if highCount > 0 {
		r.OverallRisk = SeverityHigh
	} else if mediumCount > 0 {
		r.OverallRisk = SeverityMedium
	} else if len(r.Findings) > 0 {
		r.OverallRisk = SeverityLow
	} else {
		r.OverallRisk = "Safe"
	}
}

func formatOutput(r *ScanResult, format string) string {
	switch format {
	case "json":
		return formatJSON(r)
	case "markdown":
		return formatMarkdown(r)
	case "html":
		return formatHTML(r)
	default:
		return formatText(r)
	}
}

func formatText(r *ScanResult) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("\n=== セキュリティ診断結果 ===\n"))
	sb.WriteString(fmt.Sprintf("対象URL: %s\n", r.URL))
	sb.WriteString(fmt.Sprintf("診断日時: %s\n", r.ScanTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("総合リスク: %s (スコア: %d)\n\n", r.OverallRisk, r.RiskScore))
	
	sb.WriteString("--- SSL/TLS ---\n")
	if r.SSL.Enabled {
		sb.WriteString(fmt.Sprintf("  有効: はい\n"))
		sb.WriteString(fmt.Sprintf("  有効期限: %s (残り%d日)\n", r.SSL.ValidUntil.Format("2006-01-02"), r.SSL.DaysRemaining))
		sb.WriteString(fmt.Sprintf("  発行者: %s\n", r.SSL.Issuer))
		sb.WriteString(fmt.Sprintf("  プロトコル: %s\n", r.SSL.Protocol))
	} else {
		sb.WriteString("  有効: いいえ ❌\n")
	}
	
	sb.WriteString("\n--- 検出された問題 ---\n")
	for i, f := range r.Findings {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, f.Severity, f.Title))
		sb.WriteString(fmt.Sprintf("   %s\n", f.Description))
		sb.WriteString(fmt.Sprintf("   → %s\n\n", f.Remediation))
	}
	
	return sb.String()
}

func formatMarkdown(r *ScanResult) string {
	var sb strings.Builder
	
	sb.WriteString("# セキュリティ簡易診断レポート\n\n")
	sb.WriteString(fmt.Sprintf("**対象URL:** %s  \n", r.URL))
	sb.WriteString(fmt.Sprintf("**診断日時:** %s  \n", r.ScanTime.Format("2006年01月02日 15:04")))
	sb.WriteString(fmt.Sprintf("**総合リスク:** **%s** (スコア: %d)  \n\n", r.OverallRisk, r.RiskScore))
	
	sb.WriteString("---\n\n")
	
	// Summary
	sb.WriteString("## 診断サマリー\n\n")
	sb.WriteString("| 項目 | 状態 | リスク |\n")
	sb.WriteString("|------|------|--------|\n")
	
	sslStatus := "✅ 有効"
	if !r.SSL.Enabled {
		sslStatus = "❌ 無効"
	} else if r.SSL.DaysRemaining < 30 {
		sslStatus = "⚠️ 要更新"
	}
	sb.WriteString(fmt.Sprintf("| SSL証明書 | %s | %s |\n", sslStatus, r.SSL.Risk))
	sb.WriteString(fmt.Sprintf("| HTTPヘッダー | 未設定: %d項目 | %s |\n", len(r.Headers.MissingHeaders), r.Headers.Risk))
	
	cmsStatus := "未検出"
	if r.CMS.Detected {
		cmsStatus = r.CMS.Name
		if r.CMS.VersionExposed {
			cmsStatus += " (ver露出)"
		}
	}
	sb.WriteString(fmt.Sprintf("| CMS | %s | %s |\n", cmsStatus, r.CMS.Risk))
	sb.WriteString(fmt.Sprintf("| サーバー情報 | %s | %s |\n", boolToExposed(r.ServerInfo.Exposed), r.ServerInfo.Risk))
	sb.WriteString(fmt.Sprintf("| メール認証 | SPF:%s DMARC:%s | %s |\n\n", boolToMark(r.DNS.HasSPF), boolToMark(r.DNS.HasDMARC), r.DNS.Risk))
	
	// SSL Details
	sb.WriteString("## SSL/TLS詳細\n\n")
	if r.SSL.Enabled {
		sb.WriteString(fmt.Sprintf("- **有効期限:** %s（残り%d日）\n", r.SSL.ValidUntil.Format("2006年01月02日"), r.SSL.DaysRemaining))
		sb.WriteString(fmt.Sprintf("- **発行者:** %s\n", r.SSL.Issuer))
		sb.WriteString(fmt.Sprintf("- **プロトコル:** %s\n\n", r.SSL.Protocol))
	} else {
		sb.WriteString("⛔ **SSL証明書が設定されていません**\n\n")
	}
	
	// Findings
	sb.WriteString("## 検出された問題\n\n")
	if len(r.Findings) == 0 {
		sb.WriteString("✅ 重大な問題は検出されませんでした。\n\n")
	} else {
		for i, f := range r.Findings {
			icon := "ℹ️"
			switch f.Severity {
			case "Critical":
				icon = "🔴"
			case "High":
				icon = "🟠"
			case "Medium":
				icon = "🟡"
			case "Low":
				icon = "🟢"
			}
			sb.WriteString(fmt.Sprintf("### %d. %s [%s] %s\n\n", i+1, icon, f.Severity, f.Title))
			sb.WriteString(fmt.Sprintf("**カテゴリ:** %s  \n", f.Category))
			sb.WriteString(fmt.Sprintf("**説明:** %s  \n", f.Description))
			sb.WriteString(fmt.Sprintf("**推奨対応:** %s  \n\n", f.Remediation))
		}
	}
	
	// Disclaimer
	sb.WriteString("---\n\n")
	sb.WriteString("## ご注意\n\n")
	sb.WriteString("この診断は公開情報のみを使用した簡易診断です。")
	sb.WriteString("詳細な脆弱性診断については、許可を得た上での本格診断をご検討ください。\n\n")
	sb.WriteString("**診断実施:** Trident Capital Symbiosis 合同会社\n")
	
	return sb.String()
}

func formatJSON(r *ScanResult) string {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func formatHTML(r *ScanResult) string {
	var sb strings.Builder
	
	sb.WriteString(`<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>セキュリティ簡易診断レポート</title>
<style>
body { font-family: 'Hiragino Sans', 'Meiryo', sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; line-height: 1.6; }
h1 { color: #333; border-bottom: 2px solid #333; padding-bottom: 10px; }
h2 { color: #555; margin-top: 30px; }
h3 { color: #666; }
table { border-collapse: collapse; width: 100%; margin: 15px 0; }
th, td { border: 1px solid #333; padding: 10px; text-align: left; }
th { background: #f0f0f0; }
.risk-critical { color: #d32f2f; font-weight: bold; }
.risk-high { color: #f57c00; font-weight: bold; }
.risk-medium { color: #fbc02d; font-weight: bold; }
.risk-low { color: #388e3c; }
.risk-safe { color: #1976d2; }
.finding { background: #fafafa; padding: 15px; margin: 10px 0; border-left: 4px solid #ccc; }
.finding-critical { border-left-color: #d32f2f; }
.finding-high { border-left-color: #f57c00; }
.finding-medium { border-left-color: #fbc02d; }
.finding-low { border-left-color: #388e3c; }
.meta { color: #666; }
.footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #ccc; color: #888; font-size: 0.9em; }
</style>
</head>
<body>
`)
	
	// Header
	sb.WriteString("<h1>セキュリティ簡易診断レポート</h1>\n")
	sb.WriteString(fmt.Sprintf("<p class=\"meta\"><strong>対象URL:</strong> <a href=\"%s\">%s</a><br>\n", r.URL, r.URL))
	sb.WriteString(fmt.Sprintf("<strong>診断日時:</strong> %s<br>\n", r.ScanTime.Format("2006年01月02日 15:04")))
	riskClass := "risk-" + strings.ToLower(r.OverallRisk)
	sb.WriteString(fmt.Sprintf("<strong>総合リスク:</strong> <span class=\"%s\">%s</span> (スコア: %d)</p>\n\n", riskClass, r.OverallRisk, r.RiskScore))
	
	// Summary Table
	sb.WriteString("<h2>診断サマリー</h2>\n")
	sb.WriteString("<table>\n<tr><th>項目</th><th>状態</th><th>リスク</th></tr>\n")
	
	sslStatus := "✅ 有効"
	if !r.SSL.Enabled {
		sslStatus = "❌ 無効"
	} else if r.SSL.DaysRemaining < 30 {
		sslStatus = "⚠️ 要更新"
	}
	sb.WriteString(fmt.Sprintf("<tr><td>SSL証明書</td><td>%s</td><td>%s</td></tr>\n", sslStatus, r.SSL.Risk))
	sb.WriteString(fmt.Sprintf("<tr><td>HTTPヘッダー</td><td>未設定: %d項目</td><td>%s</td></tr>\n", len(r.Headers.MissingHeaders), r.Headers.Risk))
	
	cmsStatus := "未検出"
	if r.CMS.Detected {
		cmsStatus = r.CMS.Name
		if r.CMS.VersionExposed {
			cmsStatus += " (ver露出)"
		}
	}
	sb.WriteString(fmt.Sprintf("<tr><td>CMS</td><td>%s</td><td>%s</td></tr>\n", cmsStatus, r.CMS.Risk))
	sb.WriteString(fmt.Sprintf("<tr><td>サーバー情報</td><td>%s</td><td>%s</td></tr>\n", boolToExposed(r.ServerInfo.Exposed), r.ServerInfo.Risk))
	sb.WriteString(fmt.Sprintf("<tr><td>メール認証</td><td>SPF:%s DMARC:%s</td><td>%s</td></tr>\n", boolToMark(r.DNS.HasSPF), boolToMark(r.DNS.HasDMARC), r.DNS.Risk))
	sb.WriteString("</table>\n\n")
	
	// SSL Details
	sb.WriteString("<h2>SSL/TLS詳細</h2>\n")
	if r.SSL.Enabled {
		sb.WriteString("<ul>\n")
		sb.WriteString(fmt.Sprintf("<li><strong>有効期限:</strong> %s（残り%d日）</li>\n", r.SSL.ValidUntil.Format("2006年01月02日"), r.SSL.DaysRemaining))
		sb.WriteString(fmt.Sprintf("<li><strong>発行者:</strong> %s</li>\n", r.SSL.Issuer))
		sb.WriteString(fmt.Sprintf("<li><strong>プロトコル:</strong> %s</li>\n", r.SSL.Protocol))
		sb.WriteString("</ul>\n\n")
	} else {
		sb.WriteString("<p class=\"risk-critical\">⛔ SSL証明書が設定されていません</p>\n\n")
	}
	
	// Findings
	sb.WriteString("<h2>検出された問題</h2>\n")
	if len(r.Findings) == 0 {
		sb.WriteString("<p class=\"risk-safe\">✅ 重大な問題は検出されませんでした。</p>\n")
	} else {
		for i, f := range r.Findings {
			icon := "ℹ️"
			findingClass := "finding"
			switch f.Severity {
			case "Critical":
				icon = "🔴"
				findingClass = "finding finding-critical"
			case "High":
				icon = "🟠"
				findingClass = "finding finding-high"
			case "Medium":
				icon = "🟡"
				findingClass = "finding finding-medium"
			case "Low":
				icon = "🟢"
				findingClass = "finding finding-low"
			}
			sb.WriteString(fmt.Sprintf("<div class=\"%s\">\n", findingClass))
			sb.WriteString(fmt.Sprintf("<h3>%d. %s [%s] %s</h3>\n", i+1, icon, f.Severity, f.Title))
			sb.WriteString(fmt.Sprintf("<p><strong>カテゴリ:</strong> %s<br>\n", f.Category))
			sb.WriteString(fmt.Sprintf("<strong>説明:</strong> %s<br>\n", f.Description))
			sb.WriteString(fmt.Sprintf("<strong>推奨対応:</strong> %s</p>\n", f.Remediation))
			sb.WriteString("</div>\n")
		}
	}
	
	// Footer
	sb.WriteString("<div class=\"footer\">\n")
	sb.WriteString("<h2>ご注意</h2>\n")
	sb.WriteString("<p>この診断は公開情報のみを使用した簡易診断です。詳細な脆弱性診断については、許可を得た上での本格診断をご検討ください。</p>\n")
	sb.WriteString("<p><strong>診断実施:</strong> Trident Capital Symbiosis 合同会社</p>\n")
	sb.WriteString("</div>\n")
	
	sb.WriteString("</body>\n</html>")
	
	return sb.String()
}

func boolToMark(b bool) string {
	if b {
		return "✅"
	}
	return "❌"
}

func boolToExposed(b bool) string {
	if b {
		return "露出あり"
	}
	return "非公開"
}
