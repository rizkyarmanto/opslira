package adapters

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"time"

	"opslira/backend/internal/probes"
)

// dnsblProviders matches the PHP default list; config "dnsbl_providers"
// overrides it.
var dnsblProviders = []string{"zen.spamhaus.org", "bl.spamcop.net"}

// MailServer probes MAIL_SERVER targets: SMTP/IMAP socket banner plus an
// optional DNSBL reverse-IP reputation check (MailServerProbeAdapter). Raw
// sockets replace PHP fsockopen: every reply is read like fgets(512) and
// multi-line SMTP replies end on a line whose fourth byte is not "-".
type MailServer struct{ adapterBase }

// NewMailServer builds the SMTP_DNSBL adapter.
func NewMailServer() *MailServer { return &MailServer{adapterBase: newBase()} }

func (a *MailServer) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolSMTPDNSBL
}

func (a *MailServer) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *MailServer) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

type serviceProbe struct {
	up         bool
	banner     string
	errMessage string
	metrics    map[string]any
}

func (a *MailServer) Execute(ctx context.Context) probes.ProbeResult {
	host := a.cfgString("host", "")
	if host == "" {
		return probes.Down("Missing host in config_payload", nil, nil)
	}

	service, port := a.mailServicePort()
	metrics := map[string]any{}
	var errs []string
	start := time.Now()

	var probe serviceProbe
	if service == "smtp" {
		probe = a.probeSmtp(ctx, host, port)
	} else {
		probe = a.probeImap(ctx, host, port)
	}

	metrics[service+"_"+strconv.Itoa(port)+"_up"] = probe.up
	if probe.banner != "" {
		metrics[service+"_banner"] = probe.banner
	}
	for key, value := range probe.metrics {
		metrics[key] = value
	}
	if !probe.up {
		// Prefer the real reason (connection refused, no banner) over the
		// generic "unreachable", which says nothing to an operator.
		reason := probe.errMessage
		if reason == "" {
			reason = "unreachable"
		}
		errs = append(errs, service+":"+strconv.Itoa(port)+" "+reason)
	} else if probe.errMessage != "" {
		errs = append(errs, service+":"+strconv.Itoa(port)+" "+probe.errMessage)
	}

	// The form sends "true"/"false" as text; a plain cast would turn the
	// string "false" on (PHP filter_var).
	if a.cfgBool("check_dnsbl", false) {
		for key, value := range a.dnsblCheck(ctx, host) {
			metrics[key] = value
		}
	}

	latency := round2(float64(time.Since(start).Microseconds()) / 1000)

	if len(errs) > 0 {
		return probes.Down(strings.Join(errs, "; "), metrics, msPointer(latency))
	}
	if listed, ok := metrics["is_blacklisted"].(bool); ok && listed {
		return probes.Degraded(msPointer(latency), metrics, "IP listed on DNSBL")
	}

	status := a.classifyLatency(msPointer(latency))
	return probes.ProbeResult{Status: status, ResponseTimeMs: msPointer(latency), MetricsPayload: metrics}
}

// mailServicePort: precedence of the shared Port column, legacy check_ports,
// legacy smtp_port/imap_port keys, then the 587/993 defaults.
func (a *MailServer) mailServicePort() (string, int) {
	mode := a.cfgString("mail_check_mode", "smtp")
	if mode != "smtp" && mode != "imap" {
		mode = "smtp"
	}

	port := a.cfgInt("port", 0)
	if port == 0 {
		if legacy, ok := a.config["check_ports"].(map[string]any); ok {
			port = toInt(legacy[mode], 0)
		}
	}
	if port == 0 {
		if mode == "smtp" {
			port = a.cfgInt("smtp_port", 0)
		} else {
			port = a.cfgInt("imap_port", 0)
		}
	}
	if port == 0 {
		if mode == "imap" {
			port = 993
		} else {
			port = 587
		}
	}
	return mode, port
}

// dial opens a TCP connection, upgrading to implicit TLS on the SMTPS/IMAPS
// ports (465/993); certificate verification is off, as with PHP ssl:// in a
// monitoring probe.
func (a *MailServer) dial(ctx context.Context, host string, port int, implicitTLS bool) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Duration(a.timeoutSeconds) * time.Second}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	if !implicitTLS {
		return dialer.DialContext(ctx, "tcp", address)
	}
	return tls.DialWithDialer(&dialer, "tcp", address,
		&tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // probe semantics, as in PHP
}

// probeSmtp: 220 greeting, EHLO (HELO fallback), optional STARTTLS upgrade,
// optional AUTH LOGIN (smtp_username/smtp_password).
func (a *MailServer) probeSmtp(ctx context.Context, host string, port int) serviceProbe {
	implicitTLS := port == 465
	conn, err := a.dial(ctx, host, port, implicitTLS)
	if err != nil {
		return serviceProbe{up: false, errMessage: "connect failed: " + err.Error()}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Duration(a.timeoutSeconds) * time.Second))

	reader := bufio.NewReader(conn)
	metrics := map[string]any{}

	greeting := smtpReadLine(reader)
	banner := strings.TrimSpace(greeting)
	if greeting == "" || smtpReplyCode(banner) != 220 {
		return serviceProbe{up: false, banner: banner, metrics: metrics}
	}

	ehlo := smtpEhloReply(conn, reader, host)
	if ehlo == "" || smtpReplyCode(ehlo) != 250 {
		message := "EHLO failed"
		if ehlo != "" {
			message += ": " + strings.TrimSpace(ehlo)
		}
		return serviceProbe{up: true, banner: banner, errMessage: message, metrics: metrics}
	}
	starttlsAdvertised := strings.Contains(strings.ToUpper(ehlo), "STARTTLS")
	metrics["starttls_available"] = starttlsAdvertised

	if !implicitTLS && starttlsAdvertised {
		reply := smtpDialogue(conn, reader, "STARTTLS")
		if reply == "" || smtpReplyCode(reply) != 220 {
			return serviceProbe{up: true, banner: banner, errMessage: "STARTTLS rejected", metrics: metrics}
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // probe semantics, as in PHP
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return serviceProbe{up: true, banner: banner, errMessage: "STARTTLS upgrade failed", metrics: metrics}
		}
		conn = tlsConn
		reader = bufio.NewReader(tlsConn)

		ehlo = smtpEhloReply(conn, reader, host)
		if ehlo == "" || smtpReplyCode(ehlo) != 250 {
			message := "EHLO after STARTTLS failed"
			if ehlo != "" {
				message += ": " + strings.TrimSpace(ehlo)
			}
			return serviceProbe{up: true, banner: banner, errMessage: message, metrics: metrics}
		}
	}

	if a.credNonEmpty("smtp_username", "smtp_password") {
		code := smtpAuthLogin(conn, reader, a.credString("smtp_username"), a.credString("smtp_password"))
		ok := code == 235
		metrics["smtp_auth_ok"] = ok
		if !ok {
			return serviceProbe{up: true, banner: banner, errMessage: "SMTP AUTH failed: " + strconv.Itoa(code), metrics: metrics}
		}
	}

	_, _ = conn.Write([]byte("QUIT\r\n"))
	return serviceProbe{up: true, banner: banner, metrics: metrics}
}

// probeImap: untagged "* OK" greeting, optional LOGIN (imap_username /
// imap_password).
func (a *MailServer) probeImap(ctx context.Context, host string, port int) serviceProbe {
	implicitTLS := port == 993
	conn, err := a.dial(ctx, host, port, implicitTLS)
	if err != nil {
		return serviceProbe{up: false, errMessage: "connect failed: " + err.Error()}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Duration(a.timeoutSeconds) * time.Second))

	reader := bufio.NewReader(conn)
	metrics := map[string]any{}

	greeting := imapReadLine(reader)
	banner := strings.TrimSpace(greeting)
	if greeting == "" || !strings.HasPrefix(banner, "* OK") {
		return serviceProbe{up: false, banner: banner, metrics: metrics}
	}

	tag := "a001"
	if a.credNonEmpty("imap_username", "imap_password") {
		command := tag + " LOGIN " +
			imapQuote(a.credString("imap_username")) + " " +
			imapQuote(a.credString("imap_password")) + "\r\n"
		if _, err := conn.Write([]byte(command)); err != nil {
			return serviceProbe{up: true, banner: banner, errMessage: "IMAP LOGIN failed", metrics: metrics}
		}
		line := imapReadTaggedLine(reader, tag)
		ok := line != "" && strings.HasPrefix(line, tag+" OK")
		metrics["imap_auth_ok"] = ok
		if !ok {
			return serviceProbe{up: true, banner: banner, errMessage: "IMAP LOGIN failed: " + strings.TrimSpace(line), metrics: metrics}
		}
	}

	_, _ = conn.Write([]byte(tag + " LOGOUT\r\n"))
	return serviceProbe{up: true, banner: banner, metrics: metrics}
}

// imapQuote escapes a LOGIN argument like PHP addslashes inside double quotes.
func imapQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `'`, `\'`)
	return `"` + replacer.Replace(value) + `"`
}

func smtpReplyCode(line string) int {
	if len(line) < 3 {
		return 0
	}
	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return 0
	}
	return code
}

// smtpReadLine is fgets(512) on the socket: one line, empty string on EOF.
func smtpReadLine(reader *bufio.Reader) string {
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return line
}

// smtpReadReply reads a full (possibly multi-line) SMTP reply and returns it
// concatenated; a line whose fourth byte is not "-" ends the reply.
func smtpReadReply(reader *bufio.Reader) string {
	var reply strings.Builder
	for {
		line := smtpReadLine(reader)
		if line == "" {
			break
		}
		reply.WriteString(line)
		if len(line) < 4 || line[3] != '-' {
			break
		}
	}
	if reply.Len() == 0 {
		return ""
	}
	return reply.String()
}

func smtpDialogue(conn net.Conn, reader *bufio.Reader, command string) string {
	if _, err := conn.Write([]byte(command + "\r\n")); err != nil {
		return ""
	}
	return smtpReadReply(reader)
}

// smtpEhloReply sends EHLO with the target host as identity; some servers
// reject an unresolvable .local EHLO name, so HELO is the fallback.
func smtpEhloReply(conn net.Conn, reader *bufio.Reader, host string) string {
	reply := smtpDialogue(conn, reader, "EHLO "+host)
	if reply != "" && smtpReplyCode(reply) == 250 {
		return reply
	}
	return smtpDialogue(conn, reader, "HELO "+host)
}

// smtpAuthLogin runs AUTH LOGIN and returns the final reply code (235 on
// success).
func smtpAuthLogin(conn net.Conn, reader *bufio.Reader, username, password string) int {
	reply := smtpDialogue(conn, reader, "AUTH LOGIN")
	if reply == "" || smtpReplyCode(reply) != 334 {
		return smtpReplyCode(reply)
	}
	reply = smtpDialogue(conn, reader, base64.StdEncoding.EncodeToString([]byte(username)))
	if reply == "" || smtpReplyCode(reply) != 334 {
		return smtpReplyCode(reply)
	}
	reply = smtpDialogue(conn, reader, base64.StdEncoding.EncodeToString([]byte(password)))
	if reply == "" {
		return 0
	}
	return smtpReplyCode(reply)
}

func imapReadLine(reader *bufio.Reader) string {
	return smtpReadLine(reader)
}

// imapReadTaggedLine skips untagged lines until "a001 ..." arrives.
func imapReadTaggedLine(reader *bufio.Reader, tag string) string {
	for {
		line := imapReadLine(reader)
		if line == "" {
			return ""
		}
		if strings.HasPrefix(line, tag+" ") {
			return line
		}
	}
}

// dnsblCheck resolves the host and queries the DNSBL providers with the
// reversed octets; a provider answer means the IP is listed.
func (a *MailServer) dnsblCheck(ctx context.Context, host string) map[string]any {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil || len(ips) == 0 {
		return map[string]any{"is_blacklisted": false, "listed_on": []string{}}
	}
	ip := ips[0].String()

	providers := dnsblProviders
	if configured, ok := a.config["dnsbl_providers"].([]any); ok && len(configured) > 0 {
		providers = nil
		for _, item := range configured {
			if provider := toString(item); provider != "" {
				providers = append(providers, provider)
			}
		}
	}

	octets := strings.Split(ip, ".")
	for i, j := 0, len(octets)-1; i < j; i, j = i+1, j-1 {
		octets[i], octets[j] = octets[j], octets[i]
	}
	reversed := strings.Join(octets, ".")

	var listed []string
	for _, provider := range providers {
		if _, lookupErr := net.DefaultResolver.LookupIP(ctx, "ip4", reversed+"."+provider); lookupErr == nil {
			listed = append(listed, provider)
		}
	}

	if listed == nil {
		listed = []string{}
	}
	return map[string]any{"is_blacklisted": len(listed) > 0, "listed_on": listed}
}
