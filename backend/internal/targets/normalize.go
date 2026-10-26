package targets

// normalize mirrors TargetController::normalizeTargetPayload: INTERNET
// targets are ping-only (protocol forced, icmp_only pinned server-side so a
// client cannot re-enable TCP/SNMP), everything else strips the ping-only
// flag and maps the legacy Beszel system id key to its name.
func normalize(p *TargetPayload) {
	if p.TargetType == "INTERNET" {
		p.ProtocolType = "ICMP_PING"
		if p.ConfigPayload == nil {
			p.ConfigPayload = map[string]any{}
		}
		p.ConfigPayload["icmp_only"] = true

		return
	}

	if p.ConfigPayload != nil {
		delete(p.ConfigPayload, "icmp_only")
		if p.TargetType == "BESZEL_SERVER" {
			if _, hasName := p.ConfigPayload["beszel_system_name"]; !hasName {
				if id, ok := p.ConfigPayload["beszel_system_id"]; ok {
					p.ConfigPayload["beszel_system_name"] = id
				}
			}
			delete(p.ConfigPayload, "beszel_system_id")
		}
	}
}
