package cmd

import (
	"strings"
	"testing"
)

func TestXWeightedLength(t *testing.T) {
	// "check this out " is 15 ASCII chars; the URL always collapses to 23
	// regardless of its real length (matches t.co shortening).
	urlCase := "check this out https://example.com/very/long/path?x=1"
	urlWant := 15 + 23

	// "見て " = 2 wide runes (weight 2 each = 4) + 1 space (1) = 5
	// url = 23 (fixed)
	// " 続き" = 1 space (1) + 2 wide runes (4) = 5
	mixedCase := "見て https://example.com/a 続き"
	mixedWant := 5 + 23 + 5

	cases := []struct {
		name string
		in   string
		want int
	}{
		{"plain ascii", "hello world", 11},
		{"url collapses to 23", urlCase, urlWant},
		{"japanese counts double", "こんにちは", 10},
		{"mixed japanese and url", mixedCase, mixedWant},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xWeightedLength(c.in); got != c.want {
				t.Errorf("xWeightedLength(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestSplitForX_UnderLimitReturnsSingleChunk(t *testing.T) {
	text := "短い投稿です。"
	chunks, err := splitForX(text, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 || chunks[0] != text {
		t.Fatalf("expected single unmodified chunk, got %#v", chunks)
	}
}

func TestSplitForX_OverLimitWithoutThreadErrors(t *testing.T) {
	text := strings.Repeat("あ", 200) // 200 wide runes = 400 weighted chars
	_, err := splitForX(text, false)
	if err == nil {
		t.Fatal("expected error when over limit and thread=false")
	}
}

func TestSplitForX_OverLimitWithThreadSplits(t *testing.T) {
	words := make([]string, 100)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ") // 100*4 + 99 spaces = 499 chars, well over 280

	chunks, err := splitForX(text, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if xWeightedLength(c) > xMaxPostLength {
			t.Errorf("chunk %d exceeds limit: %d chars: %q", i, xWeightedLength(c), c)
		}
		suffix := chunkSuffix(i+1, len(chunks))
		if !strings.HasSuffix(c, suffix) {
			t.Errorf("chunk %d = %q, expected suffix %q", i, c, suffix)
		}
	}
	// Rejoining the words (minus suffixes) should reproduce all original words
	// in order, i.e. splitting doesn't drop or reorder content.
	var rebuilt []string
	for i, c := range chunks {
		trimmed := strings.TrimSuffix(c, chunkSuffix(i+1, len(chunks)))
		rebuilt = append(rebuilt, strings.Fields(trimmed)...)
	}
	if len(rebuilt) != len(words) {
		t.Fatalf("expected %d words after rebuilding, got %d", len(words), len(rebuilt))
	}
}

func chunkSuffix(i, n int) string {
	return " (" + itoa(i) + "/" + itoa(n) + ")"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestSplitForX_WhitespaceOnlyOverLimitErrors(t *testing.T) {
	// Over the limit but with no actual words to split into chunks.
	_, err := splitForX(strings.Repeat(" ", 300), true)
	if err == nil {
		t.Fatal("expected error for whitespace-only content")
	}
}
