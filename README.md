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
│   ├── media.go     # mediaサブコマンド (draft, publish)
│   └── scan.go      # セキュリティ簡易診断
└── README.md
```

## Cobraの仕組み

```
biz-tools              ← rootCmd (cmd/root.go)
├── scan               ← scanCmd (cmd/scan.go) ★セキュリティ診断
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
