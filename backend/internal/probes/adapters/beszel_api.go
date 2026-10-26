package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"opslira/backend/internal/probes"
)

// Beszel cache TTL constants (BeszelApiProbeAdapter): token success lasts
// BESZEL_TOKEN_CACHE_TTL (default 43200 s), failures at most one minute so a
// single wrong password cannot lock the target out for hours. The cache key
// itself lives in probes.BeszelTokenCacheKey (shared with the targets update
// flow's ForgetBeszelToken).
const (
	beszelFailureCacheTTL = time.Minute
	beszelBodyLimit       = 2 << 20 // 2 MiB response cap for the JSON pulls
)

// BeszelAPI probes BESZEL_SERVER targets: PocketBase REST integration that
// authenticates with a static API token or a regular-user login, caches the
// JWT, then pulls system + stats records and classifies on CPU/MEM/DISK
// thresholds (BeszelApiProbeAdapter).
type BeszelAPI struct {
	adapterBase
	cache         probes.Cache // optional; nil disables token caching
	tokenCacheTTL time.Duration
}

// NewBeszelAPI builds the BESZEL_API adapter; cache may be nil.
func NewBeszelAPI(cache probes.Cache, tokenCacheTTL time.Duration) *BeszelAPI {
	if tokenCacheTTL <= 0 {
		tokenCacheTTL = 12 * time.Hour // BESZEL_TOKEN_CACHE_TTL default 43200
	}
	return &BeszelAPI{adapterBase: newBase(), cache: cache, tokenCacheTTL: tokenCacheTTL}
}

func (a *BeszelAPI) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolBeszelAPI
}

func (a *BeszelAPI) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *BeszelAPI) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

// ForgetToken drops the cached login token for hub+email; the targets update
// flow calls it when the hub URL or email changes (PHP static forgetToken).
func ForgetToken(ctx context.Context, cache probes.Cache, hubURL, email string) {
	if cache == nil {
		return
	}
	cache.Del(ctx, probes.BeszelTokenCacheKey(hubURL, email))
}

// beszelError carries the fully formatted DOWN message; unreachable reports a
// connection failure (PHP ConnectionException branch), which keeps the
// "check hub reachability" hint instead of a generic authentication failure.
type beszelError struct {
	message     string
	unreachable bool
}

func (e *beszelError) Error() string { return e.message }

func (a *BeszelAPI) Execute(ctx context.Context) probes.ProbeResult {
	hubURL := strings.TrimSuffix(strings.TrimSpace(a.cfgString("hub_url", "")), "/")
	if hubURL == "" {
		hubURL = strings.TrimSuffix(strings.TrimSpace(a.cfgString("host", "")), "/")
	}
	systemName := a.cfgString("beszel_system_name", "")
	if hubURL == "" || systemName == "" {
		return probes.Down("Missing hub_url or beszel_system_name", nil, nil)
	}

	apiToken := a.credString("api_token")
	email := a.credString("email")
	password := a.credString("password")
	if apiToken == "" && (email == "" || password == "") {
		return probes.Down("Missing Beszel credentials", nil, nil)
	}

	calls := 3
	if apiToken != "" {
		calls = 2
	}
	perCall := a.callBudget(calls)

	// PocketBase takes the raw token in the Authorization header (no Bearer
	// prefix); connect timeouts stay under the per-call budget.
	client := &http.Client{
		Timeout: time.Duration(perCall) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: a.connectBudget(perCall)}).DialContext,
		},
	}

	token, authErr := a.authToken(ctx, client, hubURL, perCall)
	if authErr != nil {
		if authErr.unreachable {
			return probes.Down(authErr.message, nil, nil)
		}
		return probes.Down("Beszel authentication failed: "+authErr.message, nil, nil)
	}

	// Resolve the human system name to a PocketBase record id.
	systemData, queryErr := a.fetchItems(ctx, client, token, hubURL, "systems query",
		"/api/collections/systems/records", url.Values{"filter": {"(name='" + systemName + "')"}, "perPage": {"5"}}, perCall)
	if queryErr != nil {
		return probes.Down(queryErr.message, nil, nil)
	}
	if len(systemData) == 0 {
		return probes.Down(fmt.Sprintf("Beszel system '%s' not found on hub", systemName), nil, nil)
	}
	system := systemData[0]
	systemID, _ := system["id"].(string)
	if systemID == "" {
		return probes.Down(fmt.Sprintf("Beszel system '%s' not found on hub", systemName), nil, nil)
	}

	// Latest stats.
	statItems, queryErr := a.fetchItems(ctx, client, token, hubURL, "stats query",
		"/api/collections/system_stats/records", url.Values{"filter": {"(system='" + systemID + "')"}, "sort": {"-created"}, "perPage": {"1"}}, perCall)
	if queryErr != nil {
		return probes.Down(queryErr.message, nil, nil)
	}
	var record map[string]any
	if len(statItems) > 0 {
		record, _ = statItems[0]["stats"].(map[string]any)
	}
	info, _ := system["info"].(map[string]any)

	metrics := map[string]any{
		"agent_status":         stringOr(system["status"], "unknown"),
		"cpu_pct":              toFloat(firstOf(record["cpu"], info["cpu"]), 0),
		"mem_pct":              toFloat(firstOf(record["mp"], info["mp"]), 0),
		"mem_used_gb":          toFloat(record["mu"], 0),
		"mem_total_gb":         toFloat(record["m"], 0),
		"disk_pct":             toFloat(firstOf(record["dp"], info["dp"]), 0),
		"disk_used_gb":         toFloat(record["du"], 0),
		"disk_total_gb":        toFloat(record["d"], 0),
		"containers_running":   toInt(firstOf(info["ct"]), 0),
		"containers_unhealthy": 0,
	}

	// Threshold-based DEGRADED classification.
	warnCPU := toFloat(a.config["warn_cpu_pct"], 85)
	warnMem := toFloat(a.config["warn_mem_pct"], 90)
	warnDisk := toFloat(a.config["warn_disk_pct"], 85)

	var breaches []string
	if cpu, _ := metrics["cpu_pct"].(float64); cpu >= warnCPU {
		breaches = append(breaches, fmt.Sprintf("CPU %g%%", cpu))
	}
	if mem, _ := metrics["mem_pct"].(float64); mem >= warnMem {
		breaches = append(breaches, fmt.Sprintf("MEM %g%%", mem))
	}
	if disk, _ := metrics["disk_pct"].(float64); disk >= warnDisk {
		breaches = append(breaches, fmt.Sprintf("DISK %g%%", disk))
	}
	if len(breaches) > 0 {
		return probes.Degraded(nil, metrics, "Threshold breach: "+strings.Join(breaches, ", "))
	}

	return probes.Up(nil, metrics, nil)
}

// callBudget splits timeout_seconds across the sequential calls so the total
// stays inside the target timeout; minimum 2 s per call.
func (a *BeszelAPI) callBudget(calls int) int {
	perCall := (a.timeoutSeconds + calls - 1) / calls
	if perCall < 2 {
		perCall = 2
	}
	return perCall
}

func (a *BeszelAPI) connectBudget(perCall int) time.Duration {
	if perCall > 3 {
		return 3 * time.Second
	}
	return time.Duration(perCall) * time.Second
}

// authToken returns the static API token without cache, or performs the
// PocketBase login with the cached JWT (PHP authToken).
func (a *BeszelAPI) authToken(ctx context.Context, client *http.Client, hubURL string, perCall int) (string, *beszelError) {
	if apiToken := a.credString("api_token"); apiToken != "" {
		return apiToken, nil
	}

	email := a.credString("email")
	password := a.credString("password")
	if email == "" || password == "" {
		return "", &beszelError{message: "missing email or password"}
	}

	// Keyed per hub + email so two Beszel targets with different logins never
	// share a JWT.
	cacheKey := probes.BeszelTokenCacheKey(hubURL, email)
	var cached beszelAuth
	if probes.CacheGetJSON(ctx, a.cache, cacheKey, &cached) {
		if cached.Error != "" {
			return "", &beszelError{message: cached.Error}
		}
		return cached.Token, nil
	}

	payload, err := json.Marshal(map[string]string{"identity": email, "password": password})
	if err != nil {
		return "", &beszelError{message: err.Error()}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		hubURL+"/api/collections/users/auth-with-password", strings.NewReader(string(payload)))
	if err != nil {
		return "", &beszelError{message: err.Error()}
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", &beszelError{
			message: fmt.Sprintf("Beszel hub unreachable during auth at %s after %ds (check hub reachability or raise timeout_seconds): %s",
				hubURL, perCall, err.Error()),
			unreachable: true,
		}
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, beszelBodyLimit))
	var parsed map[string]any
	_ = json.Unmarshal(body, &parsed)

	token := stringOr(parsed["token"], "")
	if token == "" {
		token = stringOr(parsed["authToken"], "")
	}
	if response.StatusCode/100 == 2 && token != "" {
		probes.CacheSetJSON(ctx, a.cache, cacheKey, beszelAuth{Token: token}, a.tokenCacheTTL)
		return token, nil
	}

	httpErr := fmt.Sprintf("HTTP %d: %s", response.StatusCode, limitString(string(body), 300))
	probes.CacheSetJSON(ctx, a.cache, cacheKey, beszelAuth{Error: httpErr}, beszelFailureCacheTTL)
	return "", &beszelError{message: httpErr}
}

// fetchItems performs an authorized GET and returns the "items" array; phase
// is "systems query" or "stats query" in the PHP error wording.
func (a *BeszelAPI) fetchItems(ctx context.Context, client *http.Client, token, hubURL, phase, path string, query url.Values, perCall int) ([]map[string]any, *beszelError) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, hubURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, &beszelError{message: err.Error()}
	}
	request.Header.Set("Authorization", token)

	response, err := client.Do(request)
	if err != nil {
		return nil, &beszelError{
			message: fmt.Sprintf("Beszel hub unreachable during %s at %s after %ds (check hub reachability or raise timeout_seconds): %s",
				phase, hubURL, perCall, err.Error()),
			unreachable: true,
		}
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, beszelBodyLimit))
	if response.StatusCode/100 != 2 {
		return nil, &beszelError{message: fmt.Sprintf("Beszel %s failed: HTTP %d %s",
			phase, response.StatusCode, limitString(string(body), 300))}
	}

	var parsed struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, &beszelError{message: fmt.Sprintf("Beszel %s failed: malformed response %s",
			phase, limitString(string(body), 300))}
	}
	return parsed.Items, nil
}

// beszelAuth is the cached token entry (PHP cached ['token' =>, 'error' =>]).
type beszelAuth struct {
	Token string `json:"token"`
	Error string `json:"error"`
}

func stringOr(value any, fallback string) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fallback
}

func firstOf(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
