package builtin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Fetch guard: web_fetch is the one tool that turns a model-authored
// string into an outbound network request, so the request target is
// checked before any bytes leave the process.
//
// Two layers, because neither is sufficient alone:
//
//  1. validateFetchURL rejects malformed targets, non-HTTP schemes, and
//     URLs carrying credentials.
//  2. The transport refuses to dial loopback, private, link-local,
//     CGNAT, and multicast addresses after DNS resolution, which covers
//     redirects and DNS rebinding as well as the first hop.
//
// Loopback and private destinations remain reachable on purpose for
// local-model users and for tests: AGENT_HARNESS_ALLOW_PRIVATE_FETCH=1
// opts out of the public-only rule.

const (
	webFetchTimeout      = 30 * time.Second
	webFetchDialTimeout  = 10 * time.Second
	webFetchMaxBodyBytes = 1 << 20 // 1 MiB read cap
	webFetchMaxOutput    = 10_000  // characters handed back to the model
	webFetchMaxRedirects = 5
	webFetchUserAgent    = "agent-harness (+https://github.com/BA-CalderonMorales/agent-harness)"

	// allowPrivateFetchEnv restores fetches to loopback/private networks
	// for local servers and test fixtures.
	allowPrivateFetchEnv = "AGENT_HARNESS_ALLOW_PRIVATE_FETCH"
)

// validateFetchURL checks a fetch target without touching the network.
func validateFetchURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported URL scheme %q (only http and https are allowed)", u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("URL must include a host")
	}
	if u.User != nil {
		return fmt.Errorf("URLs with embedded credentials are not allowed")
	}
	return nil
}

// allowPrivateFetch reports whether the operator opted into private
// network destinations.
func allowPrivateFetch() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(allowPrivateFetchEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// blockedFetchIP reports whether an address is outside the public
// internet. Nil and otherwise unusable addresses count as blocked.
func blockedFetchIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// 0.0.0.0/8 "this network" and 100.64.0.0/10 carrier-grade NAT
		// are not covered by the standard-library predicates.
		if v4[0] == 0 {
			return true
		}
		if v4[0] == 100 && v4[1]&0xc0 == 0x40 {
			return true
		}
	}
	return false
}

// newWebFetchClient builds a client that is safe by default: bounded
// redirects, a timeout, and a dialer that only reaches public addresses.
func newWebFetchClient(allowPrivate bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = webFetchTimeout
	if !allowPrivate {
		dialer := &net.Dialer{Timeout: webFetchDialTimeout, KeepAlive: 30 * time.Second}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialPublic(ctx, dialer, network, addr)
		}
	}
	return &http.Client{
		Timeout:   webFetchTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= webFetchMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", webFetchMaxRedirects)
			}
			if err := validateFetchURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// dialPublic resolves a host and connects only to public addresses. The
// connection is made to the resolved IP, so a second DNS answer cannot
// swap in a private target between the check and the dial.
func dialPublic(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", host, err)
	}
	var lastErr error
	for _, resolved := range ips {
		if blockedFetchIP(resolved.IP) {
			lastErr = fmt.Errorf("refusing to fetch non-public address %s (%s); set %s=1 to allow local and private hosts", host, resolved.IP, allowPrivateFetchEnv)
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses resolved for %s", host)
	}
	return nil, lastErr
}
