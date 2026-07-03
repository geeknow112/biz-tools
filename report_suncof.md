# セキュリティ簡易診断レポート

**対象URL:** https://www.suncof.co.jp  
**診断日時:** 2026年07月03日 22:27  
**総合リスク:** **Medium** (スコア: 4)  

---

## 診断サマリー

| 項目 | 状態 | リスク |
|------|------|--------|
| SSL証明書 | ✅ 有効 | High |
| HTTPヘッダー | 未設定: 0項目 | Unknown |
| CMS | 未検出 | Info |
| サーバー情報 | 非公開 | Info |
| メール認証 | SPF:❌ DMARC:❌ | Info |

## SSL/TLS詳細

- **有効期限:** 2026年10月25日（残り114日）
- **発行者:** CloudSecure RSA Domain Validation Secure Server CA 3
- **プロトコル:** TLS 1.3

## 検出された問題

### 1. 🟡 [Medium] HSTSヘッダーなし

**カテゴリ:** HTTPヘッダー  
**説明:** Strict-Transport-Securityヘッダーが設定されていません  
**推奨対応:** HSTSヘッダーを追加してHTTPS接続を強制してください  

### 2. 🟡 [Medium] X-Frame-Optionsなし

**カテゴリ:** HTTPヘッダー  
**説明:** クリックジャッキング攻撃に対して脆弱です  
**推奨対応:** X-Frame-Options: DENY または SAMEORIGIN を設定してください  

### 3. 🟢 [Low] Content-Security-Policyなし

**カテゴリ:** HTTPヘッダー  
**説明:** CSPヘッダーが設定されていません  
**推奨対応:** CSPを設定してXSS攻撃を軽減してください  

---

## ご注意

この診断は公開情報のみを使用した簡易診断です。詳細な脆弱性診断については、許可を得た上での本格診断をご検討ください。

**診断実施:** Trident Capital Symbiosis 合同会社
