package app

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/publicsuffix"
	"remgo/internal/fetch"
	"remgo/internal/model"
)

var (
	appleID    = regexp.MustCompile(`(?:^|/)id([0-9]{6,18})(?:/|$)`)
	playID     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)+$`)
	brandAffix = regexp.MustCompile(`^(?:get|try|use|join)([a-z]{3,})$`)
)

func ParseTarget(input string) (model.Target, error) {
	raw := strings.TrimSpace(input)
	if raw == "" || len(raw) > 2048 {
		return model.Target{}, errors.New("enter a website domain or an app store URL")
	}
	withScheme := raw
	if !strings.Contains(raw, "://") {
		withScheme = "https://" + raw
	}
	u, err := url.Parse(withScheme)
	if err != nil || fetch.ValidateURL(u) != nil {
		return model.Target{}, fmt.Errorf("invalid public website or store URL")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	t := model.Target{Input: raw, Kind: "website", URL: u.Scheme + "://" + host, Domain: strings.TrimPrefix(host, "www.")}
	switch host {
	case "apps.apple.com", "itunes.apple.com":
		match := appleID.FindStringSubmatch(u.Path)
		if len(match) != 2 {
			return model.Target{}, errors.New("App Store URL must contain an app ID")
		}
		return model.Target{Input: raw, Kind: "app_store", AppleID: match[1]}, nil
	case "play.google.com":
		id := u.Query().Get("id")
		if len(id) > 255 || !playID.MatchString(id) {
			return model.Target{}, errors.New("Google Play URL must contain a package ID")
		}
		return model.Target{Input: raw, Kind: "play_store", PlayID: id}, nil
	}
	t.Brand = brand(t.Domain)
	return t, nil
}

func brand(domain string) string {
	registered, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		registered = domain
	}
	label, _, _ := strings.Cut(registered, ".")
	label = brandAffix.ReplaceAllString(label, "$1")
	for _, suffix := range []string{"app", "hq"} {
		if len(label) >= len(suffix)+3 {
			label = strings.TrimSuffix(label, suffix)
		}
	}
	if label == "" {
		return domain
	}
	first, size := utf8.DecodeRuneInString(label)
	return string(unicode.ToUpper(first)) + label[size:]
}
