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
│   └── media.go     # mediaサブコマンド (draft, publish)
└── README.md
```

## Cobraの仕組み

```
biz-tools              ← rootCmd (cmd/root.go)
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
