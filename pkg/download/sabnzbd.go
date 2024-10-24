package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kasuboski/mediaz/pkg/logger"
	"go.uber.org/zap"
)

type SabnzbdClient struct {
	http   HTTPClient
	scheme string
	host   string
	apiKey string
}

func NewSabnzbdClient(http HTTPClient, scheme, host, apiKey string) DownloadClient {
	return &SabnzbdClient{
		http,
		scheme,
		host,
		apiKey,
	}
}

type AddNewsResponse struct {
	Status bool
	NZOIDS []string `json:"nzo_ids"`
}

func (c *SabnzbdClient) Add(ctx context.Context, request AddRequest) (Status, error) {
	var status Status
	log := logger.FromCtx(ctx)

	uri, err := request.Release.DownloadURL.Get()
	if err != nil {
		log.Warn("failed to get uri from release", zap.Error(err))
		return status, err
	}

	url := url.URL{
		Host:   c.host,
		Scheme: c.scheme,
		Path:   "/sabnzbd/api",
	}

	q := url.Query()
	q.Add("mode", "addurl")
	q.Add("name", uri)
	url.RawQuery = q.Encode()

	b, err := c.do(ctx, &url)
	if err != nil {
		return status, err
	}

	var response AddNewsResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return status, err
	}

	if len(response.NZOIDS) == 0 {
		return status, errors.New("no ids returned")
	}

	return c.Get(ctx, GetRequest{ID: response.NZOIDS[0]})
}

type QueueResponse struct {
	Queue Queue `json:"queue"`
}

type Queue struct {
	Status          string      `json:"status"`
	Speedlimit      string      `json:"speedlimit"`
	SpeedlimitAbs   string      `json:"speedlimit_abs"`
	Paused          bool        `json:"paused"`
	NoofslotsTotal  int64       `json:"noofslots_total"`
	Noofslots       int64       `json:"noofslots"`
	Limit           int64       `json:"limit"`
	Start           int64       `json:"start"`
	Timeleft        string      `json:"timeleft"`
	Speed           string      `json:"speed"`
	Kbpersec        string      `json:"kbpersec"`
	Size            string      `json:"size"`
	Sizeleft        string      `json:"sizeleft"`
	MB              string      `json:"mb"`
	Mbleft          string      `json:"mbleft"`
	Slots           []Slot      `json:"slots"`
	Diskspace1      string      `json:"diskspace1"`
	Diskspace2      string      `json:"diskspace2"`
	Diskspacetotal1 string      `json:"diskspacetotal1"`
	Diskspacetotal2 string      `json:"diskspacetotal2"`
	Diskspace1Norm  string      `json:"diskspace1_norm"`
	Diskspace2Norm  string      `json:"diskspace2_norm"`
	HaveWarnings    string      `json:"have_warnings"`
	PauseInt        string      `json:"pause_int"`
	LeftQuota       string      `json:"left_quota"`
	Version         string      `json:"version"`
	Finish          int64       `json:"finish"`
	CacheArt        string      `json:"cache_art"`
	CacheSize       string      `json:"cache_size"`
	Finishaction    interface{} `json:"finishaction"`
	PausedAll       bool        `json:"paused_all"`
	Quota           string      `json:"quota"`
	HaveQuota       bool        `json:"have_quota"`
}

type Slot struct {
	Status       string   `json:"status"`
	Index        int64    `json:"index"`
	Password     string   `json:"password"`
	AvgAge       string   `json:"avg_age"`
	Script       string   `json:"script"`
	DirectUnpack *string  `json:"direct_unpack"`
	MB           string   `json:"mb"`
	Mbleft       string   `json:"mbleft"`
	Mbmissing    string   `json:"mbmissing"`
	Size         string   `json:"size"`
	Sizeleft     string   `json:"sizeleft"`
	Filename     string   `json:"filename"`
	Labels       []string `json:"labels"`
	Priority     int      `json:"priority"`
	Cat          string   `json:"cat"`
	Timeleft     string   `json:"timeleft"`
	Percentage   string   `json:"percentage"`
	NzoID        string   `json:"nzo_id"`
	Unpackopts   string   `json:"unpackopts"`
}

func (c *SabnzbdClient) Get(ctx context.Context, request GetRequest) (Status, error) {
	var status Status
	ss, err := c.List(ctx)
	if err != nil {
		return status, err
	}

	for _, s := range ss {
		if s.ID == request.ID {
			return s, nil
		}
	}

	return status, errors.New("no download found")
}

func (c *SabnzbdClient) List(ctx context.Context) ([]Status, error) {
	url := url.URL{
		Host:   c.host,
		Scheme: c.scheme,
		Path:   "/sabnzbd/api",
	}

	q := url.Query()
	q.Add("mode", "queue")
	url.RawQuery = q.Encode()

	b, err := c.do(ctx, &url)
	if err != nil {
		return nil, err
	}

	var response QueueResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return nil, err
	}

	return queueToStatus(response.Queue)
}

func queueToStatus(queue Queue) ([]Status, error) {
	slots := queue.Slots
	speedDesc := queue.Speed
	split := strings.Split(speedDesc, " ")
	speedStr := split[0]
	speed, err := strconv.ParseFloat(speedStr, 64)
	if err != nil {
		speed = 0
	}

	stats := make([]Status, len(slots))
	for i, s := range slots {
		p, err := strconv.ParseFloat(s.Percentage, 64)
		if err != nil {
			p = 0
		}
		size, err := strconv.ParseFloat(s.MB, 64)
		if err != nil {
			size = 0
		}
		stats[i] = Status{
			ID:       s.NzoID,
			Name:     s.Filename,
			Progress: p,
			Size:     int64(size),
			Speed:    int64(speed),
		}
	}

	return stats, nil
}

func (c *SabnzbdClient) do(ctx context.Context, url *url.URL) ([]byte, error) {
	log := logger.FromCtx(ctx)
	if c.http == nil {
		return nil, errors.New("http client is nil")
	}

	if url == nil {
		return nil, errors.New("url is nil")
	}

	q := url.Query()
	q.Add("apikey", c.apiKey)
	q.Add("output", "json")
	url.RawQuery = q.Encode()

	u := url.String()
	log.Debugw("sabnzbd do", "url", u)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code not ok: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
