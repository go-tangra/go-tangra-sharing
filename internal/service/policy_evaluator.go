package service

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra-sharing/internal/data/ent"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharepolicy"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// EvaluatePolicies checks whether the client is allowed to access the share
// based on the configured policies. Returns nil if access is allowed,
// or an error if denied.
func EvaluatePolicies(policies []*ent.SharePolicy, clientIP string) error {
	if len(policies) == 0 {
		return nil
	}

	// Group policies by method
	byMethod := make(map[sharepolicy.Method][]*ent.SharePolicy)
	for _, p := range policies {
		byMethod[p.Method] = append(byMethod[p.Method], p)
	}

	for method, methodPolicies := range byMethod {
		// Separate whitelist and blacklist
		var whitelists, blacklists []*ent.SharePolicy
		for _, p := range methodPolicies {
			switch p.Type {
			case sharepolicy.TypeWHITELIST:
				whitelists = append(whitelists, p)
			case sharepolicy.TypeBLACKLIST:
				blacklists = append(blacklists, p)
			}
		}

		// Check blacklist: if any match, deny
		for _, p := range blacklists {
			if matchesPolicy(p, clientIP) {
				reason := p.Reason
				if reason == "" {
					reason = fmt.Sprintf("blocked by %s blacklist policy", method)
				}
				return sharingV1.ErrorShareAccessDenied(reason)
			}
		}

		// Check whitelist: if whitelists exist, at least one must match
		if len(whitelists) > 0 {
			matched := false
			for _, p := range whitelists {
				if matchesPolicy(p, clientIP) {
					matched = true
					break
				}
			}
			if !matched {
				return sharingV1.ErrorShareAccessDenied("access denied: not in %s whitelist", method)
			}
		}
	}

	return nil
}

// matchesPolicy checks if a single policy matches the current request context
func matchesPolicy(p *ent.SharePolicy, clientIP string) bool {
	switch p.Method {
	case sharepolicy.MethodIP:
		return matchIP(p.Value, clientIP)
	case sharepolicy.MethodNETWORK:
		return matchNetwork(p.Value, clientIP)
	case sharepolicy.MethodREGION:
		// Region matching requires a geo-lookup service.
		// For now, compare directly — can be enhanced with MaxMind GeoIP later.
		return matchRegion(p.Value, clientIP)
	case sharepolicy.MethodTIME:
		return matchTimeWindow(p.Value)
	case sharepolicy.MethodMAC:
		// MAC matching depends on request headers (X-Client-MAC) — limited applicability
		return false
	case sharepolicy.MethodDEVICE:
		// Device matching depends on request headers (User-Agent) — limited applicability
		return false
	default:
		return false
	}
}

// matchIP checks if the client IP matches the policy value (exact match)
func matchIP(policyValue, clientIP string) bool {
	return strings.TrimSpace(policyValue) == strings.TrimSpace(clientIP)
}

// matchNetwork checks if the client IP falls within the CIDR range
func matchNetwork(cidrStr, clientIP string) bool {
	_, cidr, err := net.ParseCIDR(strings.TrimSpace(cidrStr))
	if err != nil {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	return cidr.Contains(ip)
}

// matchRegion is a placeholder for geo-based matching.
// In a full implementation, this would use a GeoIP database to resolve
// the client IP to a country/region code and compare.
func matchRegion(_ string, _ string) bool {
	// TODO: integrate GeoIP lookup (e.g. MaxMind GeoLite2)
	return false
}

// matchTimeWindow checks if the current time falls within the specified window.
// Expected format: "HH:MM-HH:MM" (24-hour, UTC).
func matchTimeWindow(timeRange string) bool {
	parts := strings.SplitN(timeRange, "-", 2)
	if len(parts) != 2 {
		return false
	}

	now := time.Now().UTC()
	startTime, err := time.Parse("15:04", strings.TrimSpace(parts[0]))
	if err != nil {
		return false
	}
	endTime, err := time.Parse("15:04", strings.TrimSpace(parts[1]))
	if err != nil {
		return false
	}

	// Normalize to today's date
	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startTime.Hour()*60 + startTime.Minute()
	endMinutes := endTime.Hour()*60 + endTime.Minute()

	if startMinutes <= endMinutes {
		// Normal range: e.g. 09:00-17:00
		return currentMinutes >= startMinutes && currentMinutes <= endMinutes
	}
	// Overnight range: e.g. 22:00-06:00
	return currentMinutes >= startMinutes || currentMinutes <= endMinutes
}
