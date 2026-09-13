package safeurl

import (
	"net"
	"testing"
	"time"
)

// stubResolver points every hostname at a fixed set of addresses.
func stubResolver(t *testing.T, addresses ...string) {
	t.Helper()

	previous := resolver
	resolver = func(string) ([]net.IP, error) {
		ips := make([]net.IP, 0, len(addresses))
		for _, address := range addresses {
			ips = append(ips, net.ParseIP(address))
		}

		return ips, nil
	}

	t.Cleanup(func() { resolver = previous })
}

func TestValidateRejectsNonHTTPSchemes(t *testing.T) {
	for _, raw := range []string{"ftp://example.com", "file:///etc/passwd", "gopher://example.com", "example.com"} {
		if err := Validate(raw); err == nil {
			t.Errorf("expected %q to be rejected", raw)
		}
	}
}

func TestValidateRejectsCredentials(t *testing.T) {
	stubResolver(t, "93.184.216.34")

	if err := Validate("https://user:pass@example.com/hook"); err != ErrUserinfo {
		t.Errorf("expected ErrUserinfo, got %v", err)
	}
}

func TestValidateRejectsInternalLiterals(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1:9/hook",
		"http://localhost:8080/hook",
		"http://0.0.0.0/hook",
		"http://10.0.0.5/hook",
		"http://192.168.1.10/hook",
		"http://172.16.4.4/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/hook",
		"http://[fe80::1]/hook",
		"http://[fd00::1]/hook",
		"http://100.64.0.1/hook",
		"http://255.255.255.255/hook",
		"http://224.0.0.1/hook",
	}

	// localhost still goes through the resolver, point it at loopback.
	stubResolver(t, "127.0.0.1")

	for _, raw := range blocked {
		if err := Validate(raw); err == nil {
			t.Errorf("expected %q to be rejected", raw)
		}
	}
}

func TestValidateAcceptsPublicAddresses(t *testing.T) {
	stubResolver(t, "93.184.216.34")

	for _, raw := range []string{"https://example.com/hook", "http://example.com:80/hook", "https://example.com:443/hook", "https://8.8.8.8/hook"} {
		if err := Validate(raw); err != nil {
			t.Errorf("expected %q to be accepted, got %v", raw, err)
		}
	}
}

func TestValidateRejectsOtherPorts(t *testing.T) {
	stubResolver(t, "93.184.216.34")

	for _, raw := range []string{"http://example.com:8080/hook", "https://example.com:22/hook", "http://example.com:6379/hook"} {
		if err := Validate(raw); err != ErrPort {
			t.Errorf("expected ErrPort for %q, got %v", raw, err)
		}
	}
}

func TestValidateRejectsTransitionAndReservedRanges(t *testing.T) {
	blocked := []string{
		"http://[2002:0a00:0001::1]/hook", // 6to4 wrapping 10.0.0.1
		"http://[64:ff9b::7f00:1]/hook",   // NAT64 wrapping 127.0.0.1
		"http://192.0.0.1/hook",           // RFC 6890
		"http://198.18.0.1/hook",          // RFC 2544 benchmarking
		"http://198.19.255.254/hook",      // RFC 2544 benchmarking, upper half
		"http://[::ffff:10.0.0.1]/hook",   // IPv4 mapped private address
		"http://240.0.0.1/hook",           // reserved
	}

	stubResolver(t, "93.184.216.34")

	for _, raw := range blocked {
		if err := Validate(raw); err == nil {
			t.Errorf("expected %q to be rejected", raw)
		}
	}
}

func TestValidateRejectsHostResolvingToPrivateAddress(t *testing.T) {
	stubResolver(t, "10.1.2.3")

	if err := Validate("https://internal.example.com/hook"); err != ErrBlockedAddress {
		t.Errorf("expected ErrBlockedAddress, got %v", err)
	}
}

func TestValidateRejectsMixedPublicAndPrivateAnswers(t *testing.T) {
	stubResolver(t, "93.184.216.34", "127.0.0.1")

	if err := Validate("https://rebind.example.com/hook"); err != ErrBlockedAddress {
		t.Errorf("expected ErrBlockedAddress, got %v", err)
	}
}

func TestCheckIPNil(t *testing.T) {
	if err := CheckIP(nil); err != ErrBlockedAddress {
		t.Errorf("expected ErrBlockedAddress, got %v", err)
	}
}

func TestClientDialRefusesPrivateTarget(t *testing.T) {
	stubResolver(t, "127.0.0.1")

	client := Client(2 * time.Second)
	if _, err := client.Get("http://example.com/hook"); err == nil {
		t.Fatal("expected the dialer to refuse a loopback target")
	}
}
