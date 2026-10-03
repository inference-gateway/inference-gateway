package endpoints

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// AliasPattern is the set of characters allowed in an alias. Aliases are
// embedded in tool names and skill ids, so they have to survive being part of
// an identifier.
var AliasPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

var aliasSanitizer = regexp.MustCompile(`[^a-z0-9_-]+`)

// Spec pairs a configured upstream URL with the alias it is addressed by.
type Spec struct {
	Alias string
	URL   string
}

// Parse reads a comma-separated list of "alias=url" or bare "url" entries, the
// grammar shared by MCP_SERVERS and A2A_AGENTS. A missing alias is derived from
// the URL host. Aliases must match AliasPattern and be unique; kind names the
// upstream in error messages, e.g. "mcp server".
func Parse(raw, kind string) ([]Spec, error) {
	specs := make([]Spec, 0)
	seen := make(map[string]string)

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		alias, rawURL := splitEntry(entry)

		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("invalid %s url %q: expected an http(s) url", kind, rawURL)
		}

		if alias == "" {
			alias = DeriveAlias(parsed.Hostname())
		}

		if !AliasPattern.MatchString(alias) {
			return nil, fmt.Errorf("invalid %s alias %q for %s: must match %s", kind, alias, rawURL, AliasPattern)
		}
		if other, dup := seen[alias]; dup {
			return nil, fmt.Errorf("duplicate %s alias %q for %s and %s: set an explicit alias with alias=url", kind, alias, other, rawURL)
		}

		seen[alias] = rawURL
		specs = append(specs, Spec{Alias: alias, URL: rawURL})
	}

	return specs, nil
}

// splitEntry splits an "alias=url" entry. A bare URL is returned with an empty
// alias; the alias part never contains ':' or '/', which is what keeps a URL
// with '=' in its query string from being mistaken for an alias.
func splitEntry(entry string) (alias, rawURL string) {
	name, rest, ok := strings.Cut(entry, "=")
	if !ok || strings.ContainsAny(name, ":/") {
		return "", entry
	}
	return strings.TrimSpace(name), strings.TrimSpace(rest)
}

// Redact masks the password of every URL in a MCP_SERVERS or A2A_AGENTS value
// and keeps the aliases, so the raw setting can be logged.
func Redact(raw string) string {
	entries := strings.Split(raw, ",")
	for i, entry := range entries {
		alias, rawURL := splitEntry(strings.TrimSpace(entry))
		entries[i] = RedactURL(rawURL)
		if alias != "" {
			entries[i] = alias + "=" + entries[i]
		}
	}
	return strings.Join(entries, ",")
}

// RedactURL hides any basic-auth password before a URL is logged or served; an
// unparseable URL is dropped entirely rather than echoed.
func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Redacted()
}

// DeriveAlias turns a URL host into an alias, e.g. mcp.deepwiki.com ->
// mcp_deepwiki_com.
func DeriveAlias(host string) string {
	return strings.Trim(aliasSanitizer.ReplaceAllString(strings.ToLower(host), "_"), "_")
}
