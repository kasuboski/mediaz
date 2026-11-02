package newznab

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxBody = 8 << 20
const attrNS = "http://www.newznab.com/DTD/2010/feeds/attributes/"

type ProtocolError struct {
	Code       string
	Status     int
	RetryAfter time.Duration
}

func (e *ProtocolError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("newznab request failed (code %s)", e.Code)
	}
	return fmt.Sprintf("newznab HTTP request failed (status %d)", e.Status)
}
func newClient(endpoint, key string, d HTTPDoer) (*Client, error) {
	u, e := url.Parse(endpoint)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid Newznab endpoint")
	}
	if d == nil {
		d = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Client{endpoint: endpoint, apiKey: key, http: d}, nil
}
func (c *Client) request(ctx context.Context, v url.Values) ([]byte, http.Header, error) {
	u, _ := url.Parse(c.endpoint)
	q := u.Query()
	for k, vs := range v {
		for _, x := range vs {
			q.Add(k, x)
		}
	}
	if c.apiKey != "" {
		q.Set("apikey", c.apiKey)
	}
	u.RawQuery = q.Encode()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, nil, errors.New("newznab request failed")
	}
	resp, e := c.http.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errors.New("newznab request failed")
	}
	if resp == nil || resp.Body == nil {
		return nil, nil, errors.New("newznab invalid HTTP response")
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if e != nil || len(b) > maxBody {
		return nil, nil, errors.New("newznab response body invalid or too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, &ProtocolError{Status: resp.StatusCode, RetryAfter: retryDelay(resp.Header.Get("Retry-After"))}
	}
	return b, resp.Header, nil
}

const maxRetryDelay = 5 * time.Minute

func retryDelay(s string) time.Duration {
	var d time.Duration
	if n, e := strconv.ParseInt(s, 10, 64); e == nil && n >= 0 {
		if n > int64(maxRetryDelay/time.Second) {
			return maxRetryDelay
		}
		return time.Duration(n) * time.Second
	}
	if t, e := http.ParseTime(s); e == nil {
		d = time.Until(t)
		if d < 0 {
			return 0
		}
		if d > maxRetryDelay {
			return maxRetryDelay
		}
		return d
	}
	return 0
}
func strictRoot(b []byte, want string, out any) error {
	d := xml.NewDecoder(strings.NewReader(string(b)))
	var t xml.Token
	var e error
	for {
		t, e = d.Token()
		if e != nil {
			return e
		}
		skip := false
		switch x := t.(type) {
		case xml.Comment, xml.ProcInst:
			skip = true
		case xml.CharData:
			skip = strings.TrimSpace(string(x)) == ""
		}
		if !skip {
			break
		}
	}
	s, ok := t.(xml.StartElement)
	if !ok || s.Name.Local != want || s.Name.Space != "" {
		return errors.New("invalid root")
	}
	if e = d.DecodeElement(out, &s); e != nil {
		return e
	}
	for {
		t, e = d.Token()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		switch t.(type) {
		case xml.Comment, xml.ProcInst:
			continue
		case xml.CharData:
			if strings.TrimSpace(string(t.(xml.CharData))) == "" {
				continue
			}
		}
		return errors.New("trailing XML content")
	}
}

type wireError struct {
	XMLName xml.Name
	Code    string `xml:"code,attr"`
}

func checkError(b []byte) error {
	d := xml.NewDecoder(strings.NewReader(string(b)))
	var t xml.Token
	var e error
	for {
		t, e = d.Token()
		if e != nil {
			return nil
		}
		skip := false
		switch x := t.(type) {
		case xml.Comment, xml.ProcInst:
			skip = true
		case xml.CharData:
			skip = strings.TrimSpace(string(x)) == ""
		}
		if !skip {
			break
		}
	}
	s, ok := t.(xml.StartElement)
	if !ok || s.Name.Local != "error" || s.Name.Space != "" {
		return nil
	}
	var x wireError
	if e = d.DecodeElement(&x, &s); e != nil {
		return errors.New("invalid Newznab error response")
	}
	code := x.Code
	if n, e := strconv.Atoi(code); e != nil || n < 0 || n > 9999 {
		code = ""
	}
	return &ProtocolError{Code: code}
}

type capXML struct {
	Limits struct {
		Max     int `xml:"max,attr"`
		Default int `xml:"default,attr"`
	} `xml:"limits"`
	Searching struct {
		Nodes []struct {
			XMLName   xml.Name
			Available string `xml:"available,attr"`
			Params    string `xml:"supportedParams,attr"`
		} `xml:",any"`
	} `xml:"searching"`
	Categories struct {
		Nodes []struct {
			ID   string `xml:"id,attr"`
			Name string `xml:"name,attr"`
			Subs []struct {
				ID   string `xml:"id,attr"`
				Name string `xml:"name,attr"`
			} `xml:"subcat"`
		} `xml:"category"`
	} `xml:"categories"`
}

func (c *Client) caps(ctx context.Context) (*Capabilities, error) {
	b, _, e := c.request(ctx, url.Values{"t": {"caps"}})
	if e != nil {
		return nil, e
	}
	if e = checkError(b); e != nil {
		return nil, e
	}
	var x capXML
	if e = strictRoot(b, "caps", &x); e != nil {
		return nil, errors.New("invalid Newznab capabilities XML")
	}
	o := &Capabilities{Max: x.Limits.Max, Default: x.Limits.Default, Searches: map[string]SearchCapability{}}
	for _, n := range x.Searching.Nodes {
		o.Searches[n.XMLName.Local] = SearchCapability{Available: n.Available == "yes", Params: splitParams(n.Params)}
	}
	for _, n := range x.Categories.Nodes {
		c := Category{ID: n.ID, Name: n.Name}
		for _, s := range n.Subs {
			c.Subcategories = append(c.Subcategories, Category{ID: s.ID, Name: s.Name})
		}
		o.Categories = append(o.Categories, c)
	}
	return o, nil
}
func splitParams(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

type wireAttr struct {
	XMLName xml.Name
	Name    string `xml:"name,attr"`
	Value   string `xml:"value,attr"`
}
type wireItem struct {
	Title     string `xml:"title"`
	GUID      string `xml:"guid"`
	Link      string `xml:"link"`
	Enclosure struct {
		URL    string `xml:"url,attr"`
		Type   string `xml:"type,attr"`
		Length string `xml:"length,attr"`
	} `xml:"enclosure"`
	Attrs []wireAttr `xml:",any"`
}
type wireRSS struct {
	Channel struct {
		XMLName   xml.Name
		Items     []wireItem `xml:"item"`
		Responses []struct {
			XMLName xml.Name
			Offset  int  `xml:"offset,attr"`
			Total   *int `xml:"total,attr"`
		} `xml:",any"`
	} `xml:"channel"`
}

func (c *Client) search(ctx context.Context, r SearchRequest) (*Page, error) {
	typ := r.Type
	if typ == "" {
		typ = "search"
	}
	if typ != "search" && typ != "movie" && typ != "tvsearch" {
		return nil, errors.New("unsupported Newznab search type")
	}
	if r.Limit < 0 || r.Offset < 0 || (r.Season != nil && *r.Season < 0) || (r.Episode != nil && *r.Episode < 0) {
		return nil, errors.New("invalid Newznab paging or season/episode value")
	}
	v := url.Values{"t": {typ}, "extended": {"1"}, "offset": {"0"}}
	put := func(k, s string) {
		if s != "" {
			v.Set(k, s)
		}
	}
	put("q", r.Query)
	put("imdbid", r.IMDBID)
	put("tvdbid", r.TVDBID)
	put("tvmazeid", r.TVMazeID)
	put("rid", r.RID)
	put("tmdbid", r.TMDBID)
	if r.Season != nil {
		put("season", strconv.Itoa(*r.Season))
	}
	if r.Episode != nil {
		put("ep", strconv.Itoa(*r.Episode))
	}
	if len(r.Categories) > 0 {
		v.Set("cat", strings.Join(r.Categories, ","))
	}
	if r.Limit > 0 {
		v.Set("limit", strconv.Itoa(r.Limit))
	}
	if r.Offset > 0 {
		v.Set("offset", strconv.Itoa(r.Offset))
	}
	b, _, e := c.request(ctx, v)
	if e != nil {
		return nil, e
	}
	if e = checkError(b); e != nil {
		return nil, e
	}
	var x wireRSS
	if e = strictRoot(b, "rss", &x); e != nil {
		return nil, errors.New("invalid Newznab search XML")
	}
	if x.Channel.XMLName.Local != "channel" || x.Channel.XMLName.Space != "" {
		return nil, errors.New("invalid Newznab RSS channel")
	}
	p := &Page{}
	for _, r := range x.Channel.Responses {
		if r.XMLName.Space == attrNS && r.XMLName.Local == "response" {
			p.Offset = r.Offset
			if r.Total != nil {
				p.Total = *r.Total
				p.TotalKnown = true
			}
			break
		}
		if r.XMLName.Space == "" && r.XMLName.Local == "response" {
			p.Offset = r.Offset
			if r.Total != nil {
				p.Total = *r.Total
				p.TotalKnown = true
			}
			break
		}
	}
	for _, it := range x.Channel.Items {
		z := Item{Title: it.Title, GUID: it.GUID, Link: it.Link, EnclosureURL: it.Enclosure.URL, EnclosureType: it.Enclosure.Type, Attributes: map[string][]string{}}
		z.Size, _ = strconv.ParseInt(it.Enclosure.Length, 10, 64)
		if z.Size <= 0 {
			z.Size = 0
			for _, a := range it.Attrs {
				if (a.XMLName.Space == attrNS || a.XMLName.Space == "" && a.XMLName.Local == "attr") && a.Name == "size" {
					if size, err := strconv.ParseInt(a.Value, 10, 64); err == nil && size > 0 {
						z.Size = size
						break
					}
				}
			}
		}
		for _, a := range it.Attrs {
			if a.XMLName.Space != attrNS && (a.XMLName.Space != "" || a.XMLName.Local != "attr") {
				continue
			}
			k := a.Name
			if k == "" {
				continue
			}
			if k != "" {
				z.Attributes[k] = append(z.Attributes[k], a.Value)
			}
		}
		p.Items = append(p.Items, z)
	}
	return p, nil
}
