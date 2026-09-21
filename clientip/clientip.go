// Package clientip works out which address a request really came from.
//
// Behind a reverse proxy the TCP peer is the proxy, so every request looks
// like it came from the same place — and anything keyed on the client, like
// the verification-email send limit, collapses into one bucket shared by
// every user. The real address is in X-Forwarded-For, but that header is
// written by whoever sends the request, so it is only believed when the peer
// is a proxy we operate.
//
// The chain is read right to left. Each proxy appends the address it received
// the request from (nginx's $proxy_add_x_forwarded_for), so the rightmost
// entries were written by our own proxies and anything to the left of the
// first address we do not trust was supplied by the client and is ignored. A
// caller cannot dodge a limit by sending its own X-Forwarded-For: its value
// ends up to the left of the address our proxy recorded for it.
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// DefaultTrusted is used when no trusted proxies are configured: loopback and
// the private ranges. A peer on these can only be a process on the same host
// or network — nginx, the web app, a container gateway — so trusting its
// headers adds nothing an attacker who can reach it does not already have.
var DefaultTrusted = []string{
	"127.0.0.0/8",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
}

// Resolver resolves the client address of a request.
type Resolver struct {
	trusted []netip.Prefix
}

// New builds a resolver trusting the given proxies, each an IP or a CIDR.
// An empty list means DefaultTrusted; the single entry "none" trusts nothing,
// so the TCP peer is always the answer.
func New(entries []string) (*Resolver, error) {
	var cleaned []string
	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			if part = strings.TrimSpace(part); part != "" {
				cleaned = append(cleaned, part)
			}
		}
	}
	if len(cleaned) == 0 {
		cleaned = DefaultTrusted
	}
	if len(cleaned) == 1 && strings.EqualFold(cleaned[0], "none") {
		return &Resolver{}, nil
	}

	resolver := &Resolver{trusted: make([]netip.Prefix, 0, len(cleaned))}
	for _, entry := range cleaned {
		prefix, err := parsePrefix(entry)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", entry, err)
		}
		resolver.trusted = append(resolver.trusted, prefix)
	}
	return resolver, nil
}

func parsePrefix(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, err
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func (r *Resolver) isTrusted(addr netip.Addr) bool {
	for _, prefix := range r.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// FromRequest returns the client address for r as a string.
//
//   - A peer we do not trust is the client: its headers are ignored.
//   - Otherwise X-Forwarded-For is walked from the right, past every address
//     we trust; the first one we do not is the client.
//   - An entry that is not an address stops the walk — the chain is not
//     trustworthy past that point — and the furthest trusted hop reached is
//     returned rather than guessing further left.
//   - With no X-Forwarded-For, X-Real-IP from a trusted peer is used.
func (r *Resolver) FromRequest(req *http.Request) string {
	peer, ok := parseAddr(req.RemoteAddr)
	if !ok {
		return req.RemoteAddr
	}
	if !r.isTrusted(peer) {
		return peer.String()
	}

	if hops := forwardedFor(req); len(hops) > 0 {
		current := peer
		for i := len(hops) - 1; i >= 0; i-- {
			hop, ok := parseAddr(hops[i])
			if !ok {
				break
			}
			current = hop
			if !r.isTrusted(hop) {
				return hop.String()
			}
		}
		return current.String()
	}

	if real, ok := parseAddr(req.Header.Get("X-Real-IP")); ok {
		return real.String()
	}
	return peer.String()
}

// forwardedFor flattens every X-Forwarded-For header, in order.
func forwardedFor(req *http.Request) []string {
	var hops []string
	for _, value := range req.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				hops = append(hops, part)
			}
		}
	}
	return hops
}

// parseAddr accepts a bare address or host:port, bracketed IPv6 included, and
// folds IPv4-mapped IPv6 back to IPv4 so one client is one key.
func parseAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}, false
	}
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), true
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap(), true
		}
	}
	if addr, err := netip.ParseAddr(strings.Trim(value, "[]")); err == nil {
		return addr.Unmap(), true
	}
	return netip.Addr{}, false
}
