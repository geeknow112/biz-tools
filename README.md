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
│   ├── x.go         # X API v2連携 (media post -p x用、即時投稿)
│   ├── oauth1.go    # OAuth 1.0a署名 (RFC 5849, 標準ライブラリのみで実装)
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
│   ├── publish        ← mediaPublishCmd
│   └── post           ← mediaPostCmd (cmd/x.go) ★即時投稿(PRなし、現状X専用)
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

### media post - X (Twitter) への即時投稿

Zenn/Qiita/WordPressの`draft`/`publish`はGitHubのPRレビューを経由しますが、
Xは投稿した瞬間に公開される即時性のメディアなのでPRフローに馴染みません。
そのため`media post`という別サブコマンドにしています。

```bash
# 投稿内容と文字数を確認するだけ（実際には投稿しない）
biz-tools media post tweet.md -p x --dry-run

# 実際に投稿
biz-tools media post tweet.md -p x

# 280文字を超える場合、--threadを付けると自動でスレッド分割して連続投稿する
# （付けない場合はエラーで停止する）
biz-tools media post long-post.md -p x --thread
```

- 本文はMarkdownファイルをプレーンテキストとして読み込みます（改行以外の変換はしません）。
- 文字数カウントはXの重み付けルールの簡易近似です。URLは実際の長さに関わらず23文字（t.co短縮後の長さ）として計算し、日本語などの全角文字は1文字=2としてカウントします。ただし公式の`twitter-text`アルゴリズムの全エッジケースを再現したものではありません。
- `--thread`指定時は単語境界で分割し、各投稿末尾に`(1/3)`のような通し番号を付け、`reply.in_reply_to_tweet_id`で前の投稿への返信として連続投稿します。
- 認証はOAuth 1.0a User Context（API Key/Secret + Access Token/Secret の4値）。取得方法は下記「Xの認証情報を取得する」を参照してください。

#### Xの認証情報を取得する

1. [X Developer Portal](https://developer.x.com/) でアプリを作成する
2. アプリの **User authentication settings** で OAuth 1.0a を有効化し、権限を **Read and Write** にする
3. **Keys and tokens** タブで以下を取得する
   - API Key / API Key Secret（Consumer Keys）
   - Access Token / Access Token Secret（アプリ作成後に生成。権限をRead and Writeに変更した場合は再生成が必要）
4. `config.yaml`（`config.yaml.example`をコピーして作成、**Gitにはコミットしない**）に設定する

```yaml
platforms:
  x:
    api_key: "取得したAPI Key"
    api_secret: "取得したAPI Key Secret"
    access_token: "取得したAccess Token"
    access_token_secret: "取得したAccess Token Secret"
```

#### 料金・投稿上限（2026年8月時点）

X APIは2026年2月以降、新規開発者向けの無料枠（Free tier）が廃止され、**従量課金（pay-per-use）** に一本化されています。

- 無料枠：新規アカウントには提供されていません（クレジット購入が必須）
- 投稿コスト：プレーンテキスト/画像付き投稿 $0.015/件、URLを含む投稿 $0.20/件
- 旧無料枠（2026年2月より前に発行されたアカウントで移行前の場合）：月1,500投稿、24時間あたり50件程度が目安値でしたが、現在は新規発行されていません
- 現在の正確な料金・上限は [X Developer Portal](https://developer.x.com/en/portal/products) のダッシュボードで自分のアカウントの状態を必ず確認してください（アカウントの作成時期によって条件が異なる場合があります）

## 対応プラットフォーム

- **media draft/publish**（PRレビュー経由）: Zenn, Qiita, note, WordPress
- **media post**（即時投稿）: X
- **video**: Udemy動画作成ワークフロー
- **fba**: Keepa連携、商品検索

## 開発

```bash
# ビルド
go build -o biz-tools

# 実行
./biz-tools media draft test.md -p qiita
```
