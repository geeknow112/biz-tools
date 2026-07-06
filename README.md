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

### crawl - 見込みサイト発見 (SerpApi)

Google dork検索（`site:`, `inurl:`, 除外`-`等）で、PHPエラー/警告等を公開してしまっているサイトを発見します。
**Google検索結果ページを直接スクレイピングするものではなく、[SerpApi](https://serpapi.com)（Google検索結果を返すAPIサービス）経由で取得します**（無料枠250検索/月）。

> 補足: 当初はGoogle Custom Search APIで実装していましたが、2026年1月にGoogleが無料の「ウェブ全体検索」を廃止（新規作成分は50ドメイン指定に制限）したため、SerpApiに切り替えました。

事前準備:
1. https://serpapi.com でアカウント登録
2. マイページからAPIキーを取得
3. `config.yaml` の `crawl.serpapi_key` に設定

```bash
# queries.txt に1行1クエリでdork検索式を書く
biz-tools crawl -q queries.txt -o candidates.json
```

### outreach - 営業DM文面の下書き作成（送信は手動）

`scan --batch` の結果から、DM文面の下書きと（分かれば）問い合わせページのURLを作成します。
**biz-toolsはフォーム送信もメール送信も一切行いません。** 実際の送信は人間が手動で行ってください
（日本語の問い合わせフォームは「入力→確認→完了」のような多段階フォームが多く、汎用的な自動送信は現実的でないため）。

```bash
# scan --batch の結果から下書きキューを作成（問い合わせページ探索・メッセージ下書き）
biz-tools outreach queue -i scan_batch_results.json --min-risk Medium

# キューを確認
biz-tools outreach list
```

`outreach_queue.json` に、ドメインごとの問い合わせページURL（判明した場合）とメッセージ本文が保存されます。あとはこれを見ながら、各サイトへ手動でコピー＆ペーストして送信してください。

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
