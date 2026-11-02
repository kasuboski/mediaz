package manager

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func apiPathFor(implementation, path string) string {
	if implementation == "prowlarr" && path == "" {
		return "" // Prowlarr client appends its own /api/v1 route.
	}
	if implementation == "newznab" && path == "" {
		return "/api"
	}
	return path
}

func apiPathPtr(implementation, path string) *string {
	path = apiPathFor(implementation, path)
	if path == "" {
		return nil
	}
	return &path
}

func validateIndexerSource(req AddIndexerSourceRequest) error {
	if req.Implementation != "prowlarr" && req.Implementation != "newznab" {
		return fmt.Errorf("unsupported implementation %q", req.Implementation)
	}
	if req.Scheme != "http" && req.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if req.Host == "" || strings.ContainsAny(req.Host, "/?#@\\") || strings.TrimSpace(req.Host) != req.Host {
		return fmt.Errorf("host must contain only a hostname or IP address")
	}
	parsed, err := url.Parse("//" + req.Host)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Port() != "" {
		return fmt.Errorf("host must not include a port, path, query, or credentials")
	}
	hostname := parsed.Hostname()
	if strings.Contains(hostname, ":") && net.ParseIP(hostname) == nil {
		return fmt.Errorf("invalid IP address")
	}
	if req.Port != nil && (*req.Port < 1 || *req.Port > 65535) {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	path := req.APIPath
	if req.Implementation == "newznab" && path == "" {
		path = "/api"
	}
	if path != "" {
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#") {
			return fmt.Errorf("API path must be an absolute path without query or fragment")
		}
		u, err := url.ParseRequestURI(path)
		if err != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid API path")
		}
	}
	if req.Implementation == "prowlarr" && req.APIPath != "" {
		return fmt.Errorf("custom API paths are not supported for Prowlarr")
	}
	if req.Implementation == "prowlarr" && (req.APIKey == nil || *req.APIKey == "") {
		return fmt.Errorf("API key is required for Prowlarr")
	}
	return nil
}
