package cmd

import "testing"

// TestOAuth1Signature_KnownVector verifies our signature implementation
// against the classic worked example from Twitter's own OAuth 1.0a
// documentation, so we know the RFC 5849 signing logic (percent-encoding,
// parameter sorting, base string, HMAC-SHA1) is correct end-to-end.
func TestOAuth1Signature_KnownVector(t *testing.T) {
	params := map[string]string{
		"status":                 "Hello Ladies + Gentlemen, a signed OAuth request!",
		"include_entities":       "true",
		"oauth_consumer_key":     "xvz1evFS4wEEPTGEFPHBog",
		"oauth_nonce":            "kYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        "1318622958",
		"oauth_token":            "370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb",
		"oauth_version":          "1.0",
	}
	consumerSecret := "kAcSOqF21Fu85e7zjz7ZN2U4ZRhfV3WpwPAoE3Z7kBw"
	tokenSecret := "LswwdoUaIvS8ltyTt5jkRh4J50vUPVVHtR2YPi5kE"

	got := oauth1Signature("POST", "https://api.twitter.com/1.1/statuses/update.json", params, consumerSecret, tokenSecret)
	want := "hCtSmYh+iHYCEqBWrE7C7hYmtUk="

	if got != want {
		t.Errorf("oauth1Signature() = %q, want %q", got, want)
	}
}

func TestPercentEncode(t *testing.T) {
	cases := map[string]string{
		"Ladies + Gentlemen": "Ladies%20%2B%20Gentlemen",
		"An encoded string!": "An%20encoded%20string%21",
		"Dogs, Cats & Mice":  "Dogs%2C%20Cats%20%26%20Mice",
		"☺":                  "%E2%98%BA",
		"unreserved-._~":     "unreserved-._~",
	}
	for in, want := range cases {
		if got := percentEncode(in); got != want {
			t.Errorf("percentEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildOAuth1Header_WellFormed(t *testing.T) {
	header, err := buildOAuth1Header("POST", "https://api.x.com/2/tweets", nil,
		"consumerKey", "consumerSecret", "token", "tokenSecret")
	if err != nil {
		t.Fatalf("buildOAuth1Header() error = %v", err)
	}
	if header[:6] != "OAuth " {
		t.Errorf("header should start with 'OAuth ', got %q", header)
	}
	for _, field := range []string{"oauth_consumer_key=", "oauth_nonce=", "oauth_signature=", "oauth_signature_method=\"HMAC-SHA1\"", "oauth_timestamp=", "oauth_token=", "oauth_version=\"1.0\""} {
		if !containsSubstring(header, field) {
			t.Errorf("header %q missing expected field %q", header, field)
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
