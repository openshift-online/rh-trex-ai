package audit

import (
	"net"
	"strings"
)

// maxForwardedHops bounds how many X-Forwarded-For entries are kept.
const maxForwardedHops = 20

// resolveSource works out where a request came from. peerIP is the direct
// connection. forwardedFor is the X-Forwarded-For chain as sent, recorded for
// forensics but untrusted. IP is the client: X-Forwarded-For is walked from
// the right only while each hop is a trusted proxy, the same rule as nginx's
// real_ip_recursive and Envoy's trusted hops. With no trusted proxies, IP is
// the peer and the header is never believed.
func (s *Service) resolveSource(peerAddr string, xffHeaders []string, userAgent string) Source {
	peer := remoteIP(peerAddr)
	chain := splitForwarded(xffHeaders)
	src := Source{IP: peer, PeerIP: peer, ForwardedFor: chain, UserAgent: userAgent}
	if !s.trusted(peer) {
		return src
	}
	for i := len(chain) - 1; i >= 0; i-- {
		hop := net.ParseIP(chain[i])
		if hop == nil {
			break
		}
		src.IP = hop.String()
		if !s.trusted(src.IP) {
			break
		}
	}
	return src
}

func (s *Service) trusted(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range s.trustedProxies {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

func splitForwarded(headers []string) []string {
	var out []string
	for _, h := range headers {
		for _, part := range strings.Split(h, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	if len(out) > maxForwardedHops {
		out = out[len(out)-maxForwardedHops:]
	}
	return out
}

func remoteIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
