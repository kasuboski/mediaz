package newznab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSearchFixture(t *testing.T) {
	body, err := os.ReadFile("../indexer/testdata/newznab/nzbgeek/tv-page-two/response.xml")
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") != "secret" || r.URL.Query().Get("offset") != "100" || r.URL.Query().Get("extended") != "1" || r.URL.Query().Get("limit") != "100" {
			t.Errorf("query %v", r.URL.Query())
		}
		w.Write(body)
	}))
	defer s.Close()
	c, err := New(s.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Search(context.Background(), SearchRequest{Type: "search", Limit: 100, Offset: 100})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 120 || !p.TotalKnown || p.Offset != 100 || len(p.Items) != 20 {
		t.Fatalf("page %#v", p)
	}
	want := Item{Title: "The.Great.British.Baking.Show.S01E12.Masterclass.Part.2.720p.WEB.h264-PFa", GUID: "https://indexer.example.invalid/details/release-000539", Link: "https://indexer.example.invalid/api?t=get&id=release-000539&apikey=TEST_API_KEY", EnclosureURL: "https://indexer.example.invalid/api?t=get&id=release-000539&apikey=TEST_API_KEY", EnclosureType: "application/x-nzb", Size: 1142898000, Attributes: map[string][]string{"category": {"5000", "5040"}, "size": {"1142898000"}, "guid": {"release-000539"}, "tvdbid": {"353994"}, "season": {"S01"}, "episode": {"E12"}, "language": {"English"}, "grabs": {"158"}, "comments": {"0"}, "password": {"0"}, "usenetdate": {"Fri, 06 Mar 2020 22:08:14 +0000"}, "thumbsup": {"0"}, "thumbsdown": {"0"}}}
	if got := p.Items[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("item mismatch\ngot: %#v\nwant:%#v", got, want)
	}
}
func TestCapabilitiesFixture(t *testing.T) {
	body, err := os.ReadFile("../indexer/testdata/newznab/nzbgeek/caps/response.xml")
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != "caps" {
			t.Errorf("query %v", r.URL.Query())
		}
		w.Write(body)
	}))
	defer s.Close()
	c, _ := New(s.URL, "")
	got, err := c.Caps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := &Capabilities{Max: 100, Default: 100, Searches: map[string]SearchCapability{"search": {Available: true, Params: []string{"q", "group"}}, "tv-search": {Available: true, Params: []string{"q", "rid", "tvdbid", "tvmazeid", "season", "ep"}}, "movie-search": {Available: true, Params: []string{"q", "imdbid", "genre"}}, "audio-search": {Available: true, Params: []string{"q"}}}, Categories: []Category{{ID: "1000", Name: "Console", Subcategories: []Category{{ID: "1010", Name: "NDS"}, {ID: "1020", Name: "PSP"}, {ID: "1030", Name: "Wii"}, {ID: "1035", Name: "Switch"}, {ID: "1040", Name: "Xbox"}, {ID: "1050", Name: "Xbox 360"}, {ID: "1060", Name: "WiiWare/VC"}, {ID: "1070", Name: "XBOX 360 DLC"}, {ID: "1080", Name: "PS3"}, {ID: "1090", Name: "Xbox One"}, {ID: "1100", Name: "PS4"}, {ID: "1110", Name: "NSW"}}}, {ID: "2000", Name: "Movies", Subcategories: []Category{{ID: "2010", Name: "Foreign"}, {ID: "2020", Name: "Other"}, {ID: "2030", Name: "SD"}, {ID: "2040", Name: "HD"}, {ID: "2045", Name: "UHD"}, {ID: "2050", Name: "BluRay"}, {ID: "2060", Name: "3D"}}}, {ID: "3000", Name: "Audio", Subcategories: []Category{{ID: "3010", Name: "MP3"}, {ID: "3020", Name: "Video"}, {ID: "3030", Name: "Audiobook"}, {ID: "3040", Name: "Lossless"}}}, {ID: "4000", Name: "PC", Subcategories: []Category{{ID: "4010", Name: "0day"}, {ID: "4020", Name: "ISO"}, {ID: "4030", Name: "Mac"}, {ID: "4040", Name: "Mobile-Other"}, {ID: "4050", Name: "Games"}, {ID: "4060", Name: "Mobile-iOS"}, {ID: "4070", Name: "Mobile-Android"}}}, {ID: "5000", Name: "TV", Subcategories: []Category{{ID: "5020", Name: "Foreign"}, {ID: "5030", Name: "SD"}, {ID: "5040", Name: "HD"}, {ID: "5045", Name: "UHD"}, {ID: "5050", Name: "Other"}, {ID: "5060", Name: "Sport"}, {ID: "5070", Name: "Anime"}, {ID: "5080", Name: "Documentary"}}}, {ID: "6000", Name: "XXX", Subcategories: []Category{{ID: "6010", Name: "DVD"}, {ID: "6020", Name: "WMV"}, {ID: "6030", Name: "XviD"}, {ID: "6040", Name: "x264"}, {ID: "6050", Name: "Pack"}, {ID: "6060", Name: "ImgSet"}, {ID: "6070", Name: "Other"}}}, {ID: "7000", Name: "Books", Subcategories: []Category{{ID: "7010", Name: "Mags"}, {ID: "7020", Name: "Ebook"}, {ID: "7030", Name: "Comics"}}}, {ID: "8000", Name: "Other", Subcategories: []Category{{ID: "8010", Name: "Misc"}}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("caps mismatch\ngot %#v\nwant %#v", got, want)
	}
}
func TestSanitizedProtocolError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<error code="2" description="secret leaked"/>`))
	}))
	defer s.Close()
	c, _ := New(s.URL, "SECRET")
	_, err := c.Caps(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
func TestEndpointRejectsCredentialsAndQuery(t *testing.T) {
	for _, u := range []string{"ftp://example.com", "https://user:pass@example.com/api", "https://example.com/api?apikey=x", "https://example.com/api#x"} {
		if _, e := New(u, "key"); e == nil {
			t.Errorf("accepted %s", u)
		}
	}
}

func TestRejectsNonXMLAndForeignAttributeNamespace(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<rss><channel><item><title>x</title><f:attr xmlns:f="urn:foreign" name="secret" value="x"/><attr name="plain" value="ok"/></item></channel></rss>`))
	}))
	defer s.Close()
	c, _ := New(s.URL, "")
	p, err := c.Search(context.Background(), SearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Items[0].Attributes, map[string][]string{"plain": {"ok"}}) {
		t.Fatalf("attributes %#v", p.Items[0].Attributes)
	}
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`<html/>`)) }))
	defer s2.Close()
	c, _ = New(s2.URL, "")
	if _, err = c.Caps(context.Background()); err == nil {
		t.Fatal("accepted html as caps")
	}
}
func TestFallbackSizeXMLDeclarationAndEmptyChannel(t *testing.T) {
	responses := []string{
		`<?xml version="1.0"?><error code="500" description="DO_NOT_LEAK"/>`,
		`<rss><channel><item><title>fallback</title><newznab:attr xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/" name="size" value="42"/><enclosure length="bad"/></item></channel></rss>`,
		`<rss><channel/></rss>`,
	}
	i := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(responses[i])); i++ }))
	defer s.Close()
	c, _ := New(s.URL, "")
	_, err := c.Search(context.Background(), SearchRequest{})
	var pe *ProtocolError
	if !errors.As(err, &pe) || pe.Code != "500" || strings.Contains(err.Error(), "DO_NOT_LEAK") {
		t.Fatalf("expected sanitized typed error, got %v", err)
	}
	p, err := c.Search(context.Background(), SearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 1 || p.Items[0].Size != 42 {
		t.Fatalf("fallback page %#v", p)
	}
	p, err = c.Search(context.Background(), SearchRequest{})
	if err != nil || len(p.Items) != 0 {
		t.Fatalf("empty channel: %#v %v", p, err)
	}
}
func TestRetryDelayBounds(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"-1", 0}, {"999999999999999999999999999999", 0}, {"0", 0}, {"999999", maxRetryDelay},
	} {
		if d := retryDelay(tc.value); d < 0 || d != tc.want {
			t.Errorf("retryDelay(%q) = %v, want %v", tc.value, d, tc.want)
		}
	}
	if got := retryDelay(time.Now().Add(24 * time.Hour).UTC().Format(http.TimeFormat)); got > maxRetryDelay || got < 0 {
		t.Errorf("date retry delay out of bounds: %v", got)
	}
	if got := retryDelay(time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)); got != 0 {
		t.Errorf("past date retry delay = %v, want 0", got)
	}
}

func TestProtocolErrorCodesRemainTyped(t *testing.T) {
	codes := []string{"202", "201", "203"}
	i := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><error code="%s" description="private"/>`, codes[i])
		i++
	}))
	defer s.Close()
	c, _ := New(s.URL, "")
	for _, code := range codes {
		_, err := c.Search(context.Background(), SearchRequest{})
		var pe *ProtocolError
		if !errors.As(err, &pe) || pe.Code != code || strings.Contains(err.Error(), "private") {
			t.Fatalf("code %s: %v", code, err)
		}
	}
}
func TestCanceledRequestPreservesContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c, _ := NewWithHTTP("https://example.invalid/api", "key", roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, ctx.Err() }))
	_, err := c.Caps(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
