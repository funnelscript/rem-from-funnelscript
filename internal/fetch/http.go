package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const MaxBody = 4 << 20

var (
	ErrDestination = errors.New("destination must be a public HTTP(S) address on the standard port")
	ErrTooLarge    = errors.New("response exceeds the body limit")
	forbidden      = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("100::/64"),
		netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	}
)

type Client struct {
	HTTP      *http.Client
	allowTest bool
}

func New() *Client {
	dialer := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if !publicIP(ip) {
					return nil, ErrDestination
				}
			}
			var last error = ErrDestination
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			return nil, last
		},
		ForceAttemptHTTP2: true, MaxIdleConns: 24, MaxIdleConnsPerHost: 4, MaxConnsPerHost: 6,
		IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: 6 * time.Second, ResponseHeaderTimeout: 9 * time.Second,
	}
	return &Client{HTTP: &http.Client{Transport: transport, Timeout: 14 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit reached")
		}
		return ValidateURL(req.URL)
	}}}
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.Zone() != "" {
		return false
	}
	for _, p := range forbidden {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

func ValidateURL(u *url.URL) error {
	if u == nil || u.Host == "" || u.User != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrDestination
	}
	if p := u.Port(); p != "" && (u.Scheme == "https" && p != "443" || u.Scheme == "http" && p != "80") {
		return ErrDestination
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if ip, err := netip.ParseAddr(host); err == nil {
		if publicIP(ip) {
			return nil
		}
		return ErrDestination
	}
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".home.arpa") {
		return ErrDestination
	}
	for _, c := range host {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
			return ErrDestination
		}
	}
	return nil
}

func (c *Client) Get(ctx context.Context, rawURL string) (Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Response{}, ErrDestination
	}
	if !c.allowTest {
		if err := ValidateURL(u); err != nil {
			return Response{}, err
		}
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return Response{}, err
		}
		req.Header.Set("User-Agent", "REM-Go/0.1 (public marketing research)")
		req.Header.Set("Accept", "text/html, application/json, application/xml, text/plain;q=0.9, */*;q=0.3")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return Response{}, err
		}
		if attempt == 0 && (res.StatusCode == 502 || res.StatusCode == 503 || res.StatusCode == 504) {
			res.Body.Close()
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, MaxBody+1))
		res.Body.Close()
		if readErr != nil {
			return Response{}, readErr
		}
		if len(body) > MaxBody {
			return Response{}, fmt.Errorf("%w (%d bytes)", ErrTooLarge, MaxBody)
		}
		return Response{URL: res.Request.URL.String(), Status: res.StatusCode, Body: body}, nil
	}
}
