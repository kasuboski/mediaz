package faketmdb

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCatalogRoutesAndIsolation(t *testing.T) {
	a, b := New(), New()
	ts := httptest.NewServer(a)
	defer ts.Close()
	for _, path := range []string{"/3/search/movie?query=synthetic&page=1", "/3/search/tv", "/3/movie/1001", "/3/tv/2001", "/3/tv/2001/season/0", "/3/tv/2001/external_ids", "/3/tv/2001/watch/providers"} {
		resp, err := ts.Client().Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		var value any
		if err = json.NewDecoder(resp.Body).Decode(&value); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		resp.Body.Close()
	}
	if got := len(a.Requests()); got != 7 {
		t.Fatalf("recorded %d requests", got)
	}
	if len(b.Requests()) != 0 {
		t.Fatal("request state leaked between instances")
	}
}
func TestSearchFiltersWholeCatalogBeforePaginating(t *testing.T) {
	s := httptest.NewServer(New())
	defer s.Close()
	search := func(path string) map[string]any {
		t.Helper()
		r, err := s.Client().Get(s.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	first := search("/3/search/movie")
	second := search("/3/search/movie?page=2")
	rows1 := first["results"].([]any)
	rows2 := second["results"].([]any)
	if len(rows1) != 20 || len(rows2) != 1 {
		t.Fatalf("page sizes %d, %d", len(rows1), len(rows2))
	}
	if rows1[0].(map[string]any)["id"] == rows2[0].(map[string]any)["id"] {
		t.Fatal("pages overlap")
	}
	if first["total_results"] != float64(21) || second["total_pages"] != float64(2) {
		t.Fatalf("bad totals: %#v %#v", first, second)
	}
	selected := search("/3/search/movie?query=unfinished%20summer")
	if len(selected["results"].([]any)) != 1 || selected["total_results"] != float64(1) || selected["total_pages"] != float64(1) {
		t.Fatalf("filtered page one: %#v", selected)
	}
	noMatch := search("/3/search/movie?query=not-a-title")
	if len(noMatch["results"].([]any)) != 0 || noMatch["total_results"] != float64(0) {
		t.Fatalf("no match: %#v", noMatch)
	}
	out := search("/3/search/movie?page=99")
	if len(out["results"].([]any)) != 0 || out["page"] != float64(99) {
		t.Fatalf("out of range: %#v", out)
	}
	tv := search("/3/search/tv?query=lanterns")
	if len(tv["results"].([]any)) != 1 || tv["total_results"] != float64(1) {
		t.Fatalf("tv search: %#v", tv)
	}
	badPage := search("/3/search/movie?page=bad")
	if badPage["page"] != float64(1) {
		t.Fatalf("malformed page: %#v", badPage)
	}
	for _, page := range []string{fmt.Sprint(int(^uint(0) >> 1)), "9223372036854775807"} {
		large := search("/3/search/movie?page=" + page)
		if len(large["results"].([]any)) != 0 {
			t.Fatalf("large page %s returned results", page)
		}
	}
	if got := search("/3/search/movie?query=UNFINISHED%20SUMMER"); len(got["results"].([]any)) != 1 {
		t.Fatal("case-insensitive title filter did not match")
	}
}

func TestFixtureArtworkUsesRepresentativePathsAndSearchMatchesDetails(t *testing.T) {
	if err := fs.WalkDir(fixtures, "fixtures", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fixtures.ReadFile(name)
		if err != nil {
			return err
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		checkImageFields(t, value)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(New())
	defer server.Close()
	var search struct {
		Results []struct {
			ID         int    `json:"id"`
			PosterPath string `json:"poster_path"`
		} `json:"results"`
	}
	resp, err := server.Client().Get(server.URL + "/3/search/movie?query=Glass%20Harbor")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(search.Results) != 1 {
		t.Fatalf("search returned %d results", len(search.Results))
	}
	var detail struct {
		PosterPath string `json:"poster_path"`
	}
	resp, err = server.Client().Get(server.URL + "/3/movie/1001")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if search.Results[0].ID != 1001 || search.Results[0].PosterPath != detail.PosterPath {
		t.Fatalf("search poster %d %q differs from detail %q", search.Results[0].ID, search.Results[0].PosterPath, detail.PosterPath)
	}
}

func checkImageFields(t *testing.T, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if (key == "poster_path" || key == "backdrop_path" || key == "still_path" || key == "logo_path") && item != nil {
				if path, ok := item.(string); ok && strings.Contains(path, "/synthetic/") {
					t.Errorf("invented image path in %s: %s", key, path)
				}
			}
			checkImageFields(t, item)
		}
	case []any:
		for _, item := range v {
			checkImageFields(t, item)
		}
	}
}

func TestUnknownResourceAndStatusOverride(t *testing.T) {
	s := New()
	tsrv := httptest.NewServer(s)
	defer tsrv.Close()
	for _, tc := range []struct {
		path string
		want int
	}{{"/3/movie/99999", 404}, {"/3/nope", 404}} {
		r, e := tsrv.Client().Get(tsrv.URL + tc.path)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		if r.StatusCode != tc.want {
			t.Fatalf("%s: %d", tc.path, r.StatusCode)
		}
	}
	s.SetStatus("/3/movie/1001", 429)
	r, e := tsrv.Client().Get(tsrv.URL + "/3/movie/1001")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	if r.StatusCode != 429 {
		t.Fatalf("status override: %d", r.StatusCode)
	}
}

func TestMalformedRoutesAndMethods(t *testing.T) {
	ts := httptest.NewServer(New())
	defer ts.Close()
	for _, path := range []string{"/3/movie/-1", "/3/movie/1x", "/3/movie/../1001", "/3/tv/2001/season/-1", "/3/tv/2001/season/1/extra", "/3/search/movie/extra"} {
		r, err := ts.Client().Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", path, r.StatusCode)
		}
	}
	r, err := ts.Client().Post(ts.URL+"/3/movie/1001", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d", r.StatusCode)
	}
	r, err = ts.Client().Get(ts.URL + "/3/movie/1001")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q", got)
	}
}

func TestConcurrentRequestsAndInstanceOverrides(t *testing.T) {
	a, b := New(), New()
	a.SetStatus("/3/movie/1001", 503)
	ta, tb := httptest.NewServer(a), httptest.NewServer(b)
	defer ta.Close()
	defer tb.Close()
	const count = 32
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			base := ta.URL
			want := 503
			if i%2 != 0 {
				base, want = tb.URL, 200
			}
			r, err := http.Get(base + "/3/movie/1001")
			if err != nil {
				t.Error(err)
				return
			}
			r.Body.Close()
			if r.StatusCode != want {
				t.Errorf("status %d, want %d", r.StatusCode, want)
			}
		}(i)
	}
	wg.Wait()
	if len(a.Requests()) != count/2 || len(b.Requests()) != count/2 {
		t.Fatalf("recorded counts %d/%d", len(a.Requests()), len(b.Requests()))
	}
}
