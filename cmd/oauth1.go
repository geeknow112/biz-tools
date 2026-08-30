package cmd

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// This file implements OAuth 1.0a request signing (RFC 5849) from scratch
// using only the standard library, so we don't need to pull in a third-party
// OAuth dependency just for X API posting. It is used by cmd/x.go.

// percentEncode implements the RFC 3986 percent-encoding required by OAuth
// 1.0a. This differs from net/url's QueryEscape, which encodes space as "+"
// instead of "%20" and treats "~" as reserved — both wrong for OAuth 1.0a
// signature bases.
func percentEncode(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if isUnreservedOAuthByte(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isUnreservedOAuthByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

func oauth1Nonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate oauth nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// oauth1Signature computes the HMAC-SHA1 OAuth 1.0a signature for a request.
// params must contain every parameter that has to be signed: all oauth_*
// parameters plus any query-string or x-www-form-urlencoded body parameters.
// JSON request bodies (which is what this tool sends to X API v2) are NOT
// signed — only form-encoded bodies are part of the OAuth 1.0a base string.
func oauth1Signature(method, baseURL string, params map[string]string, consumerSecret, tokenSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, percentEncode(k)+"="+percentEncode(params[k]))
	}
	paramString := strings.Join(pairs, "&")

	baseString := strings.ToUpper(method) + "&" + percentEncode(baseURL) + "&" + percentEncode(paramString)
	signingKey := percentEncode(consumerSecret) + "&" + percentEncode(tokenSecret)

	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// buildOAuth1Header builds the value of the "Authorization" header for an
// OAuth 1.0a User Context request. extraParams are additional parameters
// that must be included in the signature (query string params, or
// form-encoded body params) — pass nil for a JSON-body POST like ours.
func buildOAuth1Header(method, baseURL string, extraParams map[string]string, consumerKey, consumerSecret, token, tokenSecret string) (string, error) {
	nonce, err := oauth1Nonce()
	if err != nil {
		return "", err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	oauthParams := map[string]string{
		"oauth_consumer_key":     consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            token,
		"oauth_version":          "1.0",
	}

	allParams := make(map[string]string, len(oauthParams)+len(extraParams))
	for k, v := range oauthParams {
		allParams[k] = v
	}
	for k, v := range extraParams {
		allParams[k] = v
	}

	oauthParams["oauth_signature"] = oauth1Signature(method, baseURL, allParams, consumerSecret, tokenSecret)

	headerKeys := make([]string, 0, len(oauthParams))
	for k := range oauthParams {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	parts := make([]string, 0, len(headerKeys))
	for _, k := range headerKeys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, percentEncode(k), percentEncode(oauthParams[k])))
	}
	return "OAuth " + strings.Join(parts, ", "), nil
}
