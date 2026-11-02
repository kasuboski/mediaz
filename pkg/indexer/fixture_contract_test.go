package indexer

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fixturePagination struct {
	Offset string `json:"offset" xml:"offset,attr"`
	Total  string `json:"total" xml:"total,attr"`
}

type fixtureExpectation struct {
	Root       string              `json:"root"`
	Items      int                 `json:"item_count"`
	Pagination []fixturePagination `json:"pagination"`
	Attributes map[string]int      `json:"attribute_item_counts"`
}

type fixtureAttribute struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type fixtureItem struct {
	GUID       string             `xml:"guid"`
	Title      string             `xml:"title"`
	Attributes []fixtureAttribute `xml:"attr"`
}

type fixtureResponse struct {
	XMLName xml.Name
	Channel struct {
		Items      []fixtureItem       `xml:"item"`
		Pagination []fixturePagination `xml:"response"`
	} `xml:"channel"`
}

func readFixtureJSON(t *testing.T, path string, dest any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(dest))
	require.ErrorIs(t, decoder.Decode(new(any)), io.EOF)
}

// This tests fixture integrity, not a native Newznab client. Domain-level release
// assertions belong to the adapter once it exists; generated counts alone are
// not an independent correctness oracle for that implementation.
func TestNewznabFixtureContract(t *testing.T) {
	root := filepath.Join("testdata", "newznab", "nzbgeek")
	var manifest struct {
		Version  int      `json:"format_version"`
		Provider string   `json:"provider"`
		Protocol string   `json:"protocol"`
		Source   string   `json:"source"`
		Prowlarr string   `json:"prowlarr_version"`
		Cases    []string `json:"scenarios"`
	}
	readFixtureJSON(t, filepath.Join(root, "manifest.json"), &manifest)
	require.Equal(t, 1, manifest.Version)
	require.Equal(t, "nzbgeek", manifest.Provider)
	require.Equal(t, "newznab", manifest.Protocol)
	require.Equal(t, "2.7.0.5723", manifest.Prowlarr)
	require.Equal(t, []string{"caps", "movie-text", "movie-imdb", "tv-text-mixed-metadata", "tv-text", "tv-series", "tv-season", "tv-episode", "tv-page-two"}, manifest.Cases)

	responses := make(map[string]fixtureResponse)
	for _, name := range manifest.Cases {
		t.Run(name, func(t *testing.T) {
			var request struct {
				Method string     `json:"method"`
				Scheme string     `json:"scheme"`
				Host   string     `json:"host"`
				Path   string     `json:"path"`
				Query  [][]string `json:"query"`
			}
			readFixtureJSON(t, filepath.Join(root, name, "request.json"), &request)
			require.Equal(t, "GET", request.Method)
			require.Equal(t, "https", request.Scheme)
			require.Equal(t, "indexer.example.invalid", request.Host)
			require.Equal(t, "/api", request.Path)
			keyCount := 0
			for _, pair := range request.Query {
				require.Len(t, pair, 2)
				if pair[0] == "apikey" {
					require.Equal(t, "TEST_API_KEY", pair[1])
					keyCount++
				}
			}
			require.Equal(t, 1, keyCount)
			var scenario struct {
				Status int                `json:"status"`
				Type   string             `json:"content_type"`
				Date   string             `json:"capture_date"`
				Expect fixtureExpectation `json:"expect"`
			}
			readFixtureJSON(t, filepath.Join(root, name, "scenario.json"), &scenario)
			require.Equal(t, 200, scenario.Status)
			require.Equal(t, "application/xml", scenario.Type)
			require.Equal(t, "2026-11-02", scenario.Date)
			body, err := os.ReadFile(filepath.Join(root, name, "response.xml"))
			require.NoError(t, err)
			assertFixtureURLs(t, body)
			var response fixtureResponse
			require.NoError(t, xml.Unmarshal(body, &response))
			actual := fixtureExpectation{Root: response.XMLName.Local, Items: len(response.Channel.Items), Pagination: make([]fixturePagination, 0), Attributes: make(map[string]int)}
			actual.Pagination = append(actual.Pagination, response.Channel.Pagination...)
			for _, item := range response.Channel.Items {
				require.NotEmpty(t, item.Title)
				require.NotEmpty(t, item.GUID)
				seen := make(map[string]bool)
				for _, attr := range item.Attributes {
					seen[attr.Name] = true
				}
				for attr := range seen {
					actual.Attributes[attr]++
				}
			}
			require.Equal(t, scenario.Expect, actual)
			responses[name] = response
		})
	}

	guids := func(name string) map[string]bool {
		result := make(map[string]bool)
		for _, item := range responses[name].Channel.Items {
			require.False(t, result[item.GUID], "duplicate GUID in %s", name)
			result[item.GUID] = true
		}
		return result
	}
	subset := func(smaller, larger string) {
		large := guids(larger)
		for guid := range guids(smaller) {
			require.True(t, large[guid], "%s must be a subset of %s", smaller, larger)
		}
	}
	require.Len(t, guids("movie-text"), 67)
	require.Len(t, guids("movie-imdb"), 69)
	subset("movie-text", "movie-imdb")
	require.Equal(t, guids("tv-text"), guids("tv-series"))
	require.Len(t, guids("tv-season"), 30)
	require.Len(t, guids("tv-episode"), 2)
	subset("tv-season", "tv-text")
	subset("tv-episode", "tv-season")
	filtered := func(season, episode string) map[string]bool {
		result := make(map[string]bool)
		for _, item := range responses["tv-text"].Channel.Items {
			attrs := make(map[string]string)
			for _, a := range item.Attributes {
				attrs[a.Name] = a.Value
			}
			if attrs["season"] == season && (episode == "" || attrs["episode"] == episode) {
				result[item.GUID] = true
			}
		}
		return result
	}
	require.Equal(t, filtered("S02", ""), guids("tv-season"))
	require.Equal(t, filtered("S02", "E01"), guids("tv-episode"))
	first, second := guids("tv-text"), guids("tv-page-two")
	require.Len(t, first, 100)
	require.Len(t, second, 20)
	for guid := range second {
		require.False(t, first[guid], "page overlap")
	}
	require.Equal(t, []fixturePagination{{Offset: "100", Total: "120"}}, responses["tv-page-two"].Channel.Pagination)

	counts := make(map[string]int)
	for _, item := range responses["tv-text-mixed-metadata"].Channel.Items {
		for _, a := range item.Attributes {
			if a.Name == "tvdbid" || a.Name == "season" || a.Name == "episode" {
				counts[a.Name]++
			}
		}
	}
	require.Equal(t, map[string]int{"tvdbid": 59, "season": 59, "episode": 59}, counts)
	var repeatedCategories, repeatedLanguages, paddedIMDb, prefixedEpisodes bool
	for _, response := range responses {
		for _, item := range response.Channel.Items {
			seen := make(map[string]int)
			for _, a := range item.Attributes {
				seen[a.Name]++
				paddedIMDb = paddedIMDb || a.Name == "imdb" && strings.HasPrefix(a.Value, "00")
				prefixedEpisodes = prefixedEpisodes || a.Name == "episode" && strings.HasPrefix(a.Value, "E")
			}
			repeatedCategories = repeatedCategories || seen["category"] > 1
			repeatedLanguages = repeatedLanguages || seen["language"] > 1
		}
	}
	require.True(t, repeatedCategories)
	require.True(t, repeatedLanguages)
	require.True(t, paddedIMDb)
	require.True(t, prefixedEpisodes)
}

func assertFixtureURLs(t *testing.T, body []byte) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(body))
	check := func(value string) {
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return
		}
		u, err := url.Parse(value)
		require.NoError(t, err)
		require.True(t, strings.HasSuffix(u.Hostname(), ".invalid"), "actionable fixture URL")
		require.Nil(t, u.User)
		if key := u.Query().Get("apikey"); key != "" {
			require.Equal(t, "TEST_API_KEY", key)
		}
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return
		}
		require.NoError(t, err)
		switch v := token.(type) {
		case xml.StartElement:
			for _, attr := range v.Attr {
				if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" {
					require.Contains(t, []string{"http://www.newznab.com/DTD/2010/feeds/attributes/", "http://www.w3.org/2005/Atom"}, attr.Value)
					continue
				}
				check(attr.Value)
			}
		case xml.CharData:
			check(strings.TrimSpace(string(v)))
		}
	}
}
