// Package faketmdb provides a deterministic, in-process TMDB metadata fake.
package faketmdb

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

//go:embed fixtures
var fixtures embed.FS

type Request struct{ Method, Path, Query string }

// Server owns request history and response overrides; instances are safe to use concurrently.
type Server struct {
	mu       sync.Mutex
	requests []Request
	statuses map[string]int
}

func New() *Server { return &Server{statuses: make(map[string]int)} }

// Requests returns a snapshot of requests received by this server.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// SetStatus overrides a route's response status. Pass zero to remove the override.
func (s *Server) SetStatus(path string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status == 0 {
		delete(s.statuses, path)
	} else {
		s.statuses[path] = status
	}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, Request{r.Method, r.URL.Path, r.URL.RawQuery})
	status := s.statuses[r.URL.Path]
	s.mu.Unlock()
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	validID := func(value string, positive bool) bool {
		if value == "" {
			return false
		}
		for _, c := range value {
			if c < '0' || c > '9' {
				return false
			}
		}
		n, err := strconv.ParseUint(value, 10, 64)
		return err == nil && (!positive || n > 0)
	}
	var file string
	switch {
	case path == "3/search/movie" || path == "3/search/tv":
		kind := parts[2]
		file = fmt.Sprintf("fixtures/search-%s-1.json", kind)
	case len(parts) == 3 && parts[0] == "3" && (parts[1] == "movie" || parts[1] == "tv") && validID(parts[2], true):
		file = "fixtures/" + parts[1] + "-" + parts[2] + ".json"
	case len(parts) == 4 && parts[0] == "3" && parts[1] == "tv" && validID(parts[2], true) && parts[3] == "external_ids":
		file = "fixtures/tv-" + parts[2] + "/external_ids.json"
	case len(parts) == 5 && parts[0] == "3" && parts[1] == "tv" && validID(parts[2], true) && parts[3] == "watch" && parts[4] == "providers":
		file = "fixtures/tv-" + parts[2] + "/watch_providers.json"
	case len(parts) == 5 && parts[0] == "3" && parts[1] == "tv" && validID(parts[2], true) && parts[3] == "season" && validID(parts[4], false):
		file = "fixtures/tv-" + parts[2] + "/season-" + parts[4] + ".json"
	default:
		http.Error(w, "unsupported fake TMDB route", http.StatusNotFound)
		return
	}
	if file == "" {
		http.Error(w, "unsupported fake TMDB route", http.StatusNotFound)
		return
	}
	b, err := fixtures.ReadFile(file)
	if err != nil {
		http.Error(w, "unknown fake TMDB resource", http.StatusNotFound)
		return
	}
	// Search from the complete embedded catalog before filtering/pagination; fixture
	// pages are storage shards, not independent query result pages.
	if strings.HasPrefix(file, "fixtures/search-") {
		var obj map[string]any
		if json.Unmarshal(b, &obj) == nil {
			kind := parts[2]
			var rows []any
			for shard := 1; ; shard++ {
				shardBytes, readErr := fixtures.ReadFile(fmt.Sprintf("fixtures/search-%s-%d.json", kind, shard))
				if readErr != nil {
					break
				}
				var page struct {
					Results []any `json:"results"`
				}
				if json.Unmarshal(shardBytes, &page) == nil {
					rows = append(rows, page.Results...)
				}
			}
			q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
			filtered := make([]any, 0, len(rows))
			for _, row := range rows {
				m, _ := row.(map[string]any)
				label, _ := m["title"].(string)
				if label == "" {
					label, _ = m["name"].(string)
				}
				if q == "" || strings.Contains(strings.ToLower(label), q) {
					filtered = append(filtered, row)
				}
			}
			page, err := strconv.Atoi(r.URL.Query().Get("page"))
			if err != nil || page < 1 {
				page = 1
			}
			total := len(filtered)
			start := total
			if page <= (total+19)/20 {
				start = (page - 1) * 20
			}
			end := start + 20
			if end > total {
				end = total
			}
			obj["results"] = filtered[start:end]
			obj["page"] = page
			obj["total_results"] = total
			totalPages := (total + 19) / 20
			obj["total_pages"] = totalPages
			b, _ = json.Marshal(obj)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
