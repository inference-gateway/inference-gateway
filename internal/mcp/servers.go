package mcp

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ReservedAlias is the alias that would shadow the selector meta-tools
// mcp_tools_get / mcp_tools_execute, so servers may not claim it.
const ReservedAlias = "tools"

// aliasPattern is the set of characters allowed in a server alias. Tool names
// are built as mcp_<alias>_<tool>, so the alias has to survive being embedded
// in a function name.
var aliasPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// aliasSanitizer replaces every character not allowed in an alias.
var aliasSanitizer = regexp.MustCompile(`[^a-z0-9_-]+`)

// ServerSpec pairs a configured MCP server URL with the alias its tools are
// namespaced under.
type ServerSpec struct {
	Alias string
	URL   string
}

// ParseServers parses the MCP_SERVERS value: a comma-separated list of
// "alias=url" or bare "url" entries. A missing alias is derived from the URL
// host. Aliases must match ^[a-z0-9_-]+$, must be unique, and may not be the
// reserved alias "tools".
func ParseServers(raw string) ([]ServerSpec, error) {
	specs := make([]ServerSpec, 0)
	seen := make(map[string]string)

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		alias, rawURL := splitServerEntry(entry)

		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("invalid mcp server url %q: expected an http(s) url", rawURL)
		}

		if alias == "" {
			alias = deriveAlias(parsed.Hostname())
		}

		if !aliasPattern.MatchString(alias) {
			return nil, fmt.Errorf("invalid mcp server alias %q for %s: must match %s", alias, rawURL, aliasPattern)
		}
		if alias == ReservedAlias {
			return nil, fmt.Errorf("mcp server alias %q is reserved (it would shadow %s/%s)", alias, SelectorToolGet, SelectorToolExecute)
		}
		if other, dup := seen[alias]; dup {
			return nil, fmt.Errorf("duplicate mcp server alias %q for %s and %s: set an explicit alias with alias=url", alias, other, rawURL)
		}

		seen[alias] = rawURL
		specs = append(specs, ServerSpec{Alias: alias, URL: rawURL})
	}

	return specs, nil
}

// splitServerEntry splits an "alias=url" entry. A bare URL is returned with an
// empty alias; the alias part never contains ':' or '/', which is what keeps a
// URL with '=' in its query string from being mistaken for an alias.
func splitServerEntry(entry string) (alias, rawURL string) {
	name, rest, ok := strings.Cut(entry, "=")
	if !ok || strings.ContainsAny(name, ":/") {
		return "", entry
	}
	return strings.TrimSpace(name), strings.TrimSpace(rest)
}

// deriveAlias turns a URL host into an alias, e.g. mcp.deepwiki.com ->
// mcp_deepwiki_com.
func deriveAlias(host string) string {
	return strings.Trim(aliasSanitizer.ReplaceAllString(strings.ToLower(host), "_"), "_")
}

// NamespacedToolName renders an MCP tool as mcp_<alias>_<tool>, the name models
// see and call.
func NamespacedToolName(alias, toolName string) string {
	return ToolNamePrefix + alias + "_" + toolName
}
