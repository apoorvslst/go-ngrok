/*Jab koi visitor browser mein kholta hai: https://gaurav.tunnel.dev:8080/api/users
Browser server ko yeh HTTP header bhejta hai:
GET /api/users HTTP/1.1
Host: gaurav.tunnel.dev:8080

IMP : Is Host header se server ko pehchanna hai ki kis tunnel ko request forward karni hai. Is technique ko Virtual Hosting bolte hain.
Humein ek function chahiye jo kisi bhi tarah ke host header se safely clean subdomain nikaal sake:

"gaurav.tunnel.dev:8080" 
→ Port hatao 
→ "gaurav"
"GAURAV.Tunnel.Dev" 
→ Lowercase karo → "gaurav"
"tunnel.dev" (bina subdomain) → Error!
"google.com" (anpadh domain) → Error!
*/

// APAN BAS EK HEADER PARSER KAR RHE 

package router

import (
	"fmt"
	"net"
	"strings"
)

// ExtractSubdomain parses the Host header and extracts the subdomain prefix.
// Example: hostHeader="gaurav.tunnel.dev:8080", rootDomain="tunnel.dev" -> "gaurav"
func ExtractSubdomain(hostHeader string, rootDomain string) (string, error) {
	// 1. Agar port number laga hai (jaise :8080), toh port strip karo
	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		// SplitHostPort tab fail hota hai jab port na ho — that's fine, original use karo
		host = hostHeader
	}

	// 2. DNS case-insensitive hota hai, isliye sab lowercase karo
	host = strings.ToLower(host)
	rootDomain = strings.ToLower(rootDomain)

	// 3. Check karo ki host humare root domain pe end hota hai ya nahi
	suffix := "." + rootDomain
	if !strings.HasSuffix(host, suffix) {
		return "", fmt.Errorf("host '%s' is not a subdomain of '%s'", host, rootDomain)
	}

	// 4. Root domain wala suffix hata do (sirf subdomain bachega)
	subdomain := strings.TrimSuffix(host, suffix)

	// 5. Check karo ki subdomain khali toh nahi reh gaya
	if subdomain == "" {
		return "", fmt.Errorf("no subdomain found in host '%s'", hostHeader)
	}

	return subdomain, nil
}