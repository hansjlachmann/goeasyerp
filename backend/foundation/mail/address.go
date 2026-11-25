package mail

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Where the server may send mail (S49): SMTP Setup's server is a setting a user enters, so the
// server must not be made to connect to itself (loopback), to the link-local network — the cloud
// metadata address 169.254.169.254 included — or to unspecified and multicast addresses; that
// would let a user probe what listens there. Private addresses (an office's own mail relay) are
// allowed unless SMTP_BLOCK_PRIVATE=true. The check is made on the addresses the name resolves
// to when the server connects, and the server connects to the address it checked (a name that
// resolves differently a moment later — DNS rebinding — does not get around it).

// EnvBlockPrivate: "true" refuses private addresses (10/8, 172.16/12, 192.168/16, 100.64/10,
// fc00::/7) as mail servers too
const EnvBlockPrivate = "SMTP_BLOCK_PRIVATE"

// AddressRefusedError: the mail server's address is one the server does not connect to
type AddressRefusedError struct {
	Host   string
	IP     net.IP
	Reason string
}

func (e *AddressRefusedError) Error() string {
	if e.Host != "" && e.Host != e.IP.String() {
		return fmt.Sprintf("the mail server %s (%s) is not allowed: %s", e.Host, e.IP, e.Reason)
	}
	return fmt.Sprintf("the mail server %s is not allowed: %s", e.IP, e.Reason)
}

// blockPrivate: whether private addresses are refused (SMTP_BLOCK_PRIVATE)
func blockPrivate() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvBlockPrivate)), "true")
}

// cgnat is the shared address space of carrier-grade NAT (RFC 6598), refused with the private ones
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// thisNetwork is 0.0.0.0/8 ("this network": it reaches the server itself)
var thisNetwork = &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)}

// CheckAddress: why the server must not connect to ip as a mail server ("" allowed)
func CheckAddress(ip net.IP, blockPrivate bool) string {
	switch {
	case ip.IsLoopback():
		return "a loopback address (this server itself)"
	case ip.IsUnspecified() || thisNetwork.Contains(ip):
		return "an unspecified address (this server itself)"
	case ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast():
		return "a link-local address (such as the cloud metadata service)"
	case ip.IsMulticast():
		return "a multicast address"
	case blockPrivate && (ip.IsPrivate() || cgnat.Contains(ip)):
		return "a private address (SMTP_BLOCK_PRIVATE is set)"
	}
	return ""
}

// CheckHost: the error when host is an IP address the server does not connect to (nil for a
// name — names are checked when the server connects)
func CheckHost(host string) error {
	ip := net.ParseIP(strings.Trim(strings.TrimSpace(host), "[]"))
	if ip == nil {
		return nil
	}
	if reason := CheckAddress(ip, blockPrivate()); reason != "" {
		return &AddressRefusedError{Host: host, IP: ip, Reason: reason}
	}
	return nil
}

// checkAddress is the rule dialChecked applies (tests allow their loopback mail server)
var checkAddress = func(ip net.IP) string { return CheckAddress(ip, blockPrivate()) }

// dialChecked resolves host, and connects to the first of its addresses the rule allows (the
// next allowed one when that fails); none allowed: an *AddressRefusedError
func dialChecked(host, port string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, strings.Trim(host, "[]"))
	if err != nil {
		return nil, err
	}
	var refused, lastErr error
	for _, a := range addrs {
		if reason := checkAddress(a.IP); reason != "" {
			if refused == nil {
				refused = &AddressRefusedError{Host: host, IP: a.IP, Reason: reason}
			}
			continue
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(a.IP.String(), port), timeout)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	if refused != nil {
		return nil, refused
	}
	return nil, fmt.Errorf("the mail server %s has no address", host)
}
