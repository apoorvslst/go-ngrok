package router

import (
	"fmt"
	"net"
	"strings"
)

/*
================================================================================
 REAL-WORLD EXAMPLE: Mail Sorting Room (Router)
================================================================================

Role Definitions:
- VISITOR: A person sending a letter addressed to "apoorv.tunnel.dev:8080".
- ROUTER (ExtractSubdomain): The postal worker who reads the envelope, strips
  the postal code (:8080) and city ("tunnel.dev"), and extracts just the
  recipient's mailbox name: "apoorv".
================================================================================
*/

// ExtractSubdomain parses the Host header and extracts the subdomain prefix.
// Example: hostHeader="apoorv.tunnel.dev:8080", rootDomain="tunnel.dev" -> "apoorv"
func ExtractSubdomain(hostHeader string, rootDomain string) (string, error) {
	// 1. Strip port number if present (e.g. "apoorv.tunnel.dev:8080" -> "apoorv.tunnel.dev")
	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		// SplitHostPort fails when there is no port (e.g. "apoorv.tunnel.dev") — that's fine
		host = hostHeader
	}

	// 2. Lowercase for case-insensitive comparison (DNS is case-insensitive)
	host = strings.ToLower(host)
	rootDomain = strings.ToLower(rootDomain)

	// 3. Check if host ends with our root domain
	suffix := "." + rootDomain
	if !strings.HasSuffix(host, suffix) {
		return "", fmt.Errorf("host '%s' is not a subdomain of '%s'", host, rootDomain)
	}

	// 4. Extract subdomain by trimming the suffix
	subdomain := strings.TrimSuffix(host, suffix)

	// 5. Validate non-empty
	if subdomain == "" {
		return "", fmt.Errorf("no subdomain found in host '%s'", hostHeader)
	}

	return subdomain, nil
}

