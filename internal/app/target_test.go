package app

import "testing"

func TestParseTarget(t *testing.T) {
	for _, tt := range []struct{ input, kind, domain, apple, play string }{
		{"https://www.notion.so/pricing?ignore=me", "website", "notion.so", "", ""},
		{"https://apps.apple.com/us/app/cal-ai/id6480417616", "app_store", "", "6480417616", ""},
		{"https://play.google.com/store/apps/details?id=com.example.product", "play_store", "", "", "com.example.product"},
		{"tryexample.co.uk", "website", "tryexample.co.uk", "", ""},
	} {
		got, err := ParseTarget(tt.input)
		if err != nil || got.Kind != tt.kind || got.Domain != tt.domain || got.AppleID != tt.apple || got.PlayID != tt.play {
			t.Errorf("%s: %+v %v", tt.input, got, err)
		}
	}
	for _, bad := range []string{"", "localhost", "http://127.0.0.1", "ftp://example.com", "https://user@example.com", "https://play.google.com/store/apps/details", "https://example.com:80"} {
		if _, err := ParseTarget(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
