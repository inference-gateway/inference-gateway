package config

// defaultMaxRequestBodySize is the fallback body-size limit used when
// MaxRequestBodySize is unset (e.g. a config built without env parsing).
const defaultMaxRequestBodySize = 10 << 20 // 10 MiB

// ResolveMaxRequestBodySize returns the configured request body limit, falling
// back to defaultMaxRequestBodySize when unset or non-positive. It is the single
// source of the fallback shared by every handler that reads a request body.
func (s *ServerConfig) ResolveMaxRequestBodySize() int {
	if s != nil && s.MaxRequestBodySize > 0 {
		return s.MaxRequestBodySize
	}
	return defaultMaxRequestBodySize
}
