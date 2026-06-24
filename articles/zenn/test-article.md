---
title: "Go + Cobraで作るCLIツール入門"
emoji: "🐍"
type: "tech"
topics: ["go", "cobra", "cli"]
published: false
---

## はじめに

この記事では、GoとCobraを使ってCLIツールを作成する方法を紹介します。

## Cobraとは

CobraはGo製のCLIフレームワークで、kubectl、hugo、gh（GitHub CLI）などで採用されています。

## 基本的な使い方

```go
package main

import (
    "fmt"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "My CLI application",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Hello, Cobra!")
    },
}

func main() {
    rootCmd.Execute()
}
```

## まとめ

Cobraを使えば、構造化されたCLIツールを簡単に作成できます。
