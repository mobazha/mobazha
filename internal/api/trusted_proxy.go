// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

const trustedProxyCIDRsEnv = "MOBAZHA_TRUSTED_PROXY_CIDRS"

type trustedProxySet []netip.Prefix

func newTrustedProxySet(configured []string) (trustedProxySet, error) {
	values := append([]string(nil), configured...)
	if len(values) == 0 {
		values = strings.FieldsFunc(os.Getenv(trustedProxyCIDRsEnv), func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
		})
	}
	set := make(trustedProxySet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy CIDR %q: %w", value, err)
		}
		set = append(set, prefix.Masked())
	}
	return set, nil
}

func (s trustedProxySet) contains(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range s {
		if prefix.Contains(addr) || prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// clientIP returns the direct peer unless that peer belongs to a configured
// trusted proxy network. For trusted chains, scan X-Forwarded-For from right
// to left and return the first untrusted hop; this prevents a client-prepended
// address from overriding the proxy-appended source address.
func (g *Gateway) clientIP(r *http.Request) string {
	direct, ok := requestRemoteAddr(r)
	if !ok {
		return remoteIP(r)
	}
	if g == nil || !g.trustedProxies.contains(direct) {
		return direct.String()
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		addresses := make([]netip.Addr, 0, len(parts))
		for _, part := range parts {
			addr, err := netip.ParseAddr(strings.TrimSpace(part))
			if err != nil {
				return direct.String()
			}
			addresses = append(addresses, addr.Unmap())
		}
		for i := len(addresses) - 1; i >= 0; i-- {
			if !g.trustedProxies.contains(addresses[i]) {
				return addresses[i].String()
			}
		}
		return direct.String()
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if addr, err := netip.ParseAddr(realIP); err == nil {
			return addr.Unmap().String()
		}
	}
	return direct.String()
}

func requestRemoteAddr(r *http.Request) (netip.Addr, bool) {
	if r == nil {
		return netip.Addr{}, false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}
