# biz-tools

Business automation CLI tool built with Go + [Cobra](https://github.com/spf13/cobra)

## 構成

```
biz-tools/
├── main.go          # エントリーポイント (cmd.Execute()を呼ぶだけ)
├── go.mod           # 依存関係 (cobra v1.10.2)
├── go.sum
├── cmd/
│   ├── root.go      # ルートコマンド定義 (cobra.Command)
│   ├── config.go    # config.yaml 読み込み (Config/PlatformConfig/CrawlConfig/OutreachConfig)
│   ├── media.go     # mediaサブコマンド (draft, publish)
│   ├── wp.go        # WordPress REST API連携 (media publish -p wordpress用)
│   ├── scan.go      # セキュリティ簡易診断 (単体 / --batch)
│   ├── crawl.go     # Google Custom Search APIでの見込みサイト発見
│   └── outreach.go  # 問い合わせフォーム経由の営業DM (要承認)
└── README.md
```

## Cobraの仕組み

```
biz-tools              ← rootCmd (cmd/root.go)
├── scan               ← scanCmd (cmd/scan.go) ★セキュリティ診断
├── crawl              ← crawlCmd (cmd/crawl.go) ★見込みサイト発見
├── outreach           ← outreachCmd (cmd/outreach.go) ★営業DM(要承認)
│   ├── queue          ← outreachQueueCmd
│   ├── list           ← outreachListCmd
│   ├── approve        ← outreachApproveCmd
│   ├── send           ← outreachSendCmd
│   └── history        ← outreachHistoryCmd
├── media              ← mediaCmd (cmd/media.go)
│   ├── draft          ← mediaDraftCmd
│   └── publish        ← mediaPublishCmd
├── video              ← (予定)
└── fba                ← (予定)
```

各コマンドは `&cobra.Command{}` で定義し、`rootCmd.AddCommand()` で親子関係を構築。

## インストール

```bash
go install github.com/geeknow112/biz-tools@latest
```

## 使い方

### scan - セキュリティ簡易診断

Webサイトの公開情報のみを使用したセキュリティ簡易診断を実行します。

```bash
# 基本的な使い方
biz-tools scan https://example.com

# Markdown形式で出力
biz-tools scan https://example.com -o markdown

# JSON形式で出力
biz-tools scan https://example.com -o json

# ファイルに保存
biz-tools scan https://example.com -o markdown -f report.md
```

#### チェック項目

| 項目 | 説明 |
|------|------|
| SSL証明書 | 有効性、有効期限、プロトコル |
| HTTPヘッダー | HSTS, X-Frame-Options, CSP等 |
| CMS検出 | WordPress, Drupal, Joomla |
| サーバー情報 | Server, X-Powered-By の露出 |
| DNS | SPF, DMARC レコード |

#### オプション

| オプション | 説明 | デフォルト |
|-----------|------|----------|
| `-o, --output` | 出力形式 (text, markdown, json) | text |
| `-f, --file` | 出力ファイルパス | - |
| `-t, --timeout` | タイムアウト秒数 | 10 |
| `-b, --batch` | crawlの出力(JSON/CSV)を一括診断 | - |

```bash
# crawlの結果を一括診断
biz-tools scan --batch candidates.json -f scan_batch_results.json
```

### crawl - 見込みサイト発見 (Google Custom Search API)

Google dork検索（`site:`, `inurl:`, 除外`-`等）で、PHPエラー/警告等を公開してしまっているサイトを発見します。
**Google検索結果ページを直接スクレイピングするものではなく、公式のCustom Search JSON APIを使用します**（無料枠1日100クエリ）。

事前準備:
1. https://console.cloud.google.com/apis/credentials でAPIキーを発行
2. https://programmablesearchengine.google.com/ で検索エンジンを作成（「ウェブ全体を検索」に設定）
3. `config.yaml` の `crawl.google_api_key` / `crawl.google_cse_id` に設定

```bash
# queries.txt に1行1クエリでdork検索式を書く
biz-tools crawl -q queries.txt -o candidates.json
```

### outreach - 営業DM (要承認・自動送信ではない)

`scan --batch` の結果から、問い合わせフォームを自動検出してDM案を作成します。
**人間が明示的に承認したものだけが送信されます。** 完全自動送信ではありません。

```bash
# 1. scan --batch の結果からキューを作成（フォーム自動検出・メッセージ下書き）
biz-tools outreach queue -i scan_batch_results.json --min-risk Medium

# 2. キューを確認
biz-tools outreach list

# 3. 送ってよいものだけ承認（ドメイン指定 or --all）
biz-tools outreach approve example.co.jp
biz-tools outreach approve --all

# 4. 承認済みのみ送信（--dry-runで送信内容だけ確認可能）
biz-tools outreach send --dry-run
biz-tools outreach send

# 5. 送信履歴（次回queue作成時に重複送信を自動的にスキップ）
biz-tools outreach history
```

フォームが自動検出できなかったサイトは `manual_required: true` としてキューに残り、`send` では自動的にスキップされます（手動対応が必要）。

### media - 記事管理

```bash
# ヘルプ表示
biz-tools --help
biz-tools media --help

# 記事ドラフト作成 → GitHub PR
biz-tools media draft article.md -p zenn

# 記事公開
biz-tools media publish article.md -p zenn
```

## 対応プラットフォーム (予定)

- **media**: Zenn, Qiita, note, WordPress, X
- **video**: Udemy動画作成ワークフロー
- **fba**: Keepa連携、商品検索

## 開発

```bash
# ビルド
go build -o biz-tools

# 実行
./biz-tools media draft test.md -p qiita
```
