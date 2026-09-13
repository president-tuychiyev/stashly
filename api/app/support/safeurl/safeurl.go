// Package safeurl validates outbound webhook URLs and builds an HTTP client
// that refuses to talk to anything but public, routable addresses.
//
// Two layers protect against SSRF:
//
//  1. Validate() is called when a callback_url is accepted, so an obviously
//     internal target is rejected with a 422 before anything is stored.
//  2. Client() re-checks the resolved IP inside DialContext, which closes the
//     DNS rebinding window between validation and the actual request, and
//     re-validates every redirect target through CheckRedirect.
package safeurl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrScheme is returned for anything that is not http or https.
	ErrScheme = errors.New("callback_url must use http or https")
	// ErrHost is returned when the URL carries no host at all.
	ErrHost = errors.New("callback_url must contain a host")
	// ErrUserinfo is returned when credentials are embedded in the URL.
	ErrUserinfo = errors.New("callback_url must not contain credentials")
	// ErrUnresolvable is returned when the host has no DNS record.
	ErrUnresolvable = errors.New("callback_url host could not be resolved")
	// ErrBlockedAddress is returned when the host points at a non public IP.
	ErrBlockedAddress = errors.New("callback_url must point at a public address")
	// ErrPort is returned for any port but the two HTTP ones.
	ErrPort = errors.New("callback_url must use port 80 or 443")
)

// blockedRanges are the networks a callback may never reach. Matching on a
// mask rather than on the leading bytes keeps the boundaries exact and makes
// the list readable next to the RFC that defines each entry.
var blockedRanges = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",       // RFC 1122 "this network"
		"10.0.0.0/8",      // RFC 1918
		"100.64.0.0/10",   // RFC 6598 carrier grade NAT
		"127.0.0.0/8",     // loopback
		"169.254.0.0/16",  // RFC 3927 link local, carries the cloud metadata service
		"172.16.0.0/12",   // RFC 1918
		"192.0.0.0/24",    // RFC 6890 IETF protocol assignments
		"192.0.2.0/24",    // RFC 5737 documentation
		"192.88.99.0/24",  // RFC 7526 former 6to4 relay anycast
		"192.168.0.0/16",  // RFC 1918
		"198.18.0.0/15",   // RFC 2544 benchmarking
		"198.51.100.0/24", // RFC 5737 documentation
		"203.0.113.0/24",  // RFC 5737 documentation
		"224.0.0.0/4",     // multicast
		"240.0.0.0/4",     // RFC 1112 reserved, includes 255.255.255.255
		"::/128",          // unspecified
		"::1/128",         // loopback
		"64:ff9b::/96",    // RFC 6052 NAT64, reaches IPv4 space
		"64:ff9b:1::/48",  // RFC 8215 local use NAT64
		"100::/64",        // RFC 6666 discard only
		"2001:db8::/32",   // documentation
		"2002::/16",       // RFC 3056 6to4, embeds an arbitrary IPv4 address
		"fc00::/7",        // unique local
		"fe80::/10",       // link local
		"ff00::/8",        // multicast
	}

	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("safeurl: bad blocked range " + cidr)
		}
		networks = append(networks, network)
	}

	return networks
}()

// allowedPorts are the only ports a callback may target. Anything else is a
// service that has no business being reachable through a webhook.
var allowedPorts = map[string]bool{"": true, "80": true, "443": true}

// resolver is swapped in tests so no real DNS lookup is needed.
var resolver = func(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}

// Parse validates a raw callback URL and returns it parsed.
func Parse(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("callback_url is not a valid url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrScheme
	}
	if parsed.User != nil {
		return nil, ErrUserinfo
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, ErrHost
	}
	if !allowedPorts[parsed.Port()] {
		return nil, ErrPort
	}

	// A literal IP never needs a lookup.
	if ip := net.ParseIP(host); ip != nil {
		if err := CheckIP(ip); err != nil {
			return nil, err
		}

		return parsed, nil
	}

	ips, err := resolver(host)
	if err != nil || len(ips) == 0 {
		return nil, ErrUnresolvable
	}
	for _, ip := range ips {
		if err := CheckIP(ip); err != nil {
			return nil, err
		}
	}

	return parsed, nil
}

// Validate reports whether a callback URL may be stored and called later.
func Validate(raw string) error {
	_, err := Parse(raw)

	return err
}

// CheckIP rejects every address that is not globally routable: loopback,
// private (RFC 1918 / RFC 4193), link-local, multicast, unspecified, the
// reserved and documentation ranges, and the transition ranges (6to4, NAT64)
// that would otherwise smuggle an internal IPv4 address inside an IPv6
// literal.
func CheckIP(ip net.IP) error {
	if ip == nil {
		return ErrBlockedAddress
	}

	// An IPv4 address wrapped in an IPv6 literal is judged as the IPv4 address
	// it really is, so ::ffff:10.0.0.1 cannot slip past the v4 ranges.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() {
		return ErrBlockedAddress
	}

	for _, network := range blockedRanges {
		if network.Contains(ip) {
			return ErrBlockedAddress
		}
	}

	// 2002::/16 carries the target IPv4 address in the next 32 bits, so the
	// embedded address has to pass on its own too.
	if len(ip) == net.IPv6len && ip[0] == 0x20 && ip[1] == 0x02 {
		return CheckIP(net.IPv4(ip[2], ip[3], ip[4], ip[5]))
	}

	return nil
}

// Client builds an HTTP client that can only reach public addresses. The
// resolved IP is checked again at dial time and every redirect target is
// re-validated.
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}

	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}

			ips, err := resolver(host)
			if err != nil || len(ips) == 0 {
				return nil, ErrUnresolvable
			}

			var lastErr error
			for _, ip := range ips {
				if err := CheckIP(ip); err != nil {
					return nil, err
				}

				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}

			return nil, lastErr
		},
		TLSHandshakeTimeout: timeout,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}

			return Validate(req.URL.String())
		},
	}
}
