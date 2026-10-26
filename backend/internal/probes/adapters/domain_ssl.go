package adapters

import (
	"context"
	"crypto/sha1" //nolint:gosec // rate-limit cache key only, matches PHP sha1()
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"opslira/backend/internal/probes"
)

// whoisCacheTTL matches the PHP Cache::remember TTL of 43200 s (12 h): the
// probe interval floor is far lower and a whois server should not be hammered
// on every check.
const whoisCacheTTL = 12 * time.Hour

var (
	whoisExpiryPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)Registry Expiry Date:\s*(.+)`),
		regexp.MustCompile(`(?i)Expiry Date:\s*(.+)`),
		regexp.MustCompile(`(?i)expires:\s*(.+)`),
	}
	whoisDateLayouts = []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02",
		"02-Jan-2006",
		"02.01.2006",
		"Mon Jan 2 15:04:05 2006",
	}
)

// DomainSSL probes DOMAIN targets: DNS A lookup, SSL certificate expiry via a
// TLS handshake, and WHOIS expiry over TCP 43 (DomainSslProbeAdapter).
type DomainSSL struct {
	adapterBase
	cache probes.Cache // optional: Redis rate limit for WHOIS; nil disables
}

// NewDomainSSL builds the WHOIS_SSL adapter; cache may be nil.
func NewDomainSSL(cache probes.Cache) *DomainSSL {
	return &DomainSSL{adapterBase: newBase(), cache: cache}
}

func (a *DomainSSL) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolWHOISSSL
}

func (a *DomainSSL) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *DomainSSL) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

func (a *DomainSSL) Execute(ctx context.Context) probes.ProbeResult {
	domain := a.cfgString("domain", "")
	if domain == "" {
		domain = a.cfgString("host", "")
	}
	if domain == "" {
		return probes.Down("Missing domain in config_payload", nil, nil)
	}

	sslPort := a.cfgInt("check_ssl_port", 443)
	warnDays := a.cfgInt("warn_days_before", 14)

	metrics := map[string]any{}
	var errs []string

	// DNS resolution.
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", domain)
	resolvedIP := ""
	if err == nil && len(ips) > 0 {
		resolvedIP = ips[0].String()
	}
	metrics["dns_resolved_ip"] = resolvedIP
	if resolvedIP == "" {
		errs = append(errs, "DNS A record not found")
	}

	// SSL certificate expiry.
	sslDaysLeft, sslErr := a.sslDaysRemaining(ctx, domain, sslPort)
	if sslDaysLeft != nil {
		metrics["ssl_days_left"] = *sslDaysLeft
		metrics["ssl_expiry_date"] = time.Now().AddDate(0, 0, *sslDaysLeft).Format("2006-01-02")
	} else {
		if sslErr == "" {
			sslErr = "SSL handshake failed"
		}
		errs = append(errs, sslErr)
	}

	// WHOIS expiry (best-effort: the reason lands in metrics, never in the
	// DOWN message, so a silent whois server cannot fail the target).
	whois := a.whoisExpiry(ctx, domain)
	if whois.DaysLeft != nil {
		metrics["domain_days_left"] = *whois.DaysLeft
		metrics["domain_expiry_date"] = whois.ExpiryDate
		metrics["domain_status"] = "active"
	}
	if whois.ErrorMessage != "" {
		metrics["whois_error"] = whois.ErrorMessage
	}

	if len(errs) > 0 {
		return probes.Down(strings.Join(errs, "; "), metrics, nil)
	}

	// DEGRADED when SSL or domain is close to expiry.
	minDays := 1 << 62
	if sslDaysLeft != nil && *sslDaysLeft < minDays {
		minDays = *sslDaysLeft
	}
	if whois.DaysLeft != nil && *whois.DaysLeft < minDays {
		minDays = *whois.DaysLeft
	}
	if minDays <= warnDays {
		reason := fmt.Sprintf("Expiry within %d days (min: %dd)", warnDays, minDays)
		return probes.Degraded(nil, metrics, reason)
	}

	return probes.Up(nil, metrics, nil)
}

// sslDaysRemaining: TLS handshake with verification disabled (a monitoring
// probe checks reachability and expiry, not identity — PHP uses
// verify_peer=false) and NotAfter maths.
func (a *DomainSSL) sslDaysRemaining(ctx context.Context, domain string, port int) (*int, string) {
	dialer := net.Dialer{Timeout: time.Duration(a.timeoutSeconds) * time.Second}
	conn, err := tls.DialWithDialer(&dialer, "tcp", net.JoinHostPort(domain, strconv.Itoa(port)),
		&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // probe semantics, as in PHP
	if err != nil {
		return nil, "SSL handshake failed: " + err.Error()
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, "SSL handshake failed: no peer certificate offered"
	}

	validTo := state.PeerCertificates[0].NotAfter
	days := int(validTo.Sub(time.Now()).Hours() / 24)
	return &days, ""
}

// whoisResult is cached as JSON with the PHP field names (days_left,
// expiry_date, error) so a warm cache reads the same shape both ways.
type whoisResult struct {
	DaysLeft     *int   `json:"days_left"`
	ExpiryDate   string `json:"expiry_date"`
	ErrorMessage string `json:"error"`
}

// whoisExpiry queries the whois server over TCP 43, rate-limited to once per
// 12 h per domain through the cache (PHP Cache::remember('probe:whois:...')).
func (a *DomainSSL) whoisExpiry(ctx context.Context, domain string) whoisResult {
	cacheKey := "probe:whois:" + sha1Hex(domain)

	var cached whoisResult
	if probes.CacheGetJSON(ctx, a.cache, cacheKey, &cached) {
		return cached
	}

	result := a.whoisLookup(ctx, domain)
	probes.CacheSetJSON(ctx, a.cache, cacheKey, result, whoisCacheTTL)
	return result
}

func (a *DomainSSL) whoisLookup(ctx context.Context, domain string) whoisResult {
	tld := domain[strings.LastIndex(domain, ".")+1:]
	server := a.cfgString("whois_server", "")
	if server == "" {
		server = "whois." + tld
	}

	dialer := net.Dialer{Timeout: time.Duration(a.timeoutSeconds) * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server, "43"))
	if err != nil {
		return whoisResult{ErrorMessage: "WHOIS lookup failed: " + err.Error()}
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(time.Duration(a.timeoutSeconds) * time.Second))
	if _, err := conn.Write([]byte(domain + "\r\n")); err != nil {
		return whoisResult{ErrorMessage: "WHOIS lookup failed: " + err.Error()}
	}

	var reply strings.Builder
	buffer := make([]byte, 128)
	for {
		n, readErr := conn.Read(buffer)
		reply.Write(buffer[:n])
		if readErr != nil {
			break
		}
	}

	for _, pattern := range whoisExpiryPatterns {
		match := pattern.FindStringSubmatch(reply.String())
		if len(match) < 2 {
			continue
		}
		if expiry := parseWhoisDate(strings.TrimSpace(match[1])); expiry != nil {
			days := int(expiry.Sub(time.Now()).Hours() / 24)
			return whoisResult{DaysLeft: &days, ExpiryDate: expiry.Format("2006-01-02")}
		}
	}

	return whoisResult{ErrorMessage: "WHOIS lookup failed: no expiry date in the reply from " + server}
}

func parseWhoisDate(value string) *time.Time {
	for _, layout := range whoisDateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed
		}
	}
	return nil
}

func sha1Hex(value string) string {
	sum := sha1.Sum([]byte(value)) //nolint:gosec // cache key only, matches PHP sha1()
	return hex.EncodeToString(sum[:])
}
