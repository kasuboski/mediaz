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
	NzoIDs []string `json:"nzo_ids"`
	Status bool
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

	if len(response.NzoIDs) == 0 {
		return status, errors.New("no ids returned")
	}

	return c.Get(ctx, GetRequest{ID: response.NzoIDs[0]})
}

type QueueResponse struct {
	Queue Queue `json:"queue"`
}

type Queue struct {
	Finishaction    interface{} `json:"finishaction"`
	Diskspace1      string      `json:"diskspace1"`
	PauseInt        string      `json:"pause_int"`
	Quota           string      `json:"quota"`
	Status          string      `json:"status"`
	Speedlimit      string      `json:"speedlimit"`
	CacheSize       string      `json:"cache_size"`
	Diskspace2      string      `json:"diskspace2"`
	Timeleft        string      `json:"timeleft"`
	Speed           string      `json:"speed"`
	Kbpersec        string      `json:"kbpersec"`
	Size            string      `json:"size"`
	Sizeleft        string      `json:"sizeleft"`
	MB              string      `json:"mb"`
	Mbleft          string      `json:"mbleft"`
	CacheArt        string      `json:"cache_art"`
	Version         string      `json:"version"`
	LeftQuota       string      `json:"left_quota"`
	Diskspacetotal1 string      `json:"diskspacetotal1"`
	Diskspacetotal2 string      `json:"diskspacetotal2"`
	Diskspace1Norm  string      `json:"diskspace1_norm"`
	Diskspace2Norm  string      `json:"diskspace2_norm"`
	HaveWarnings    string      `json:"have_warnings"`
	SpeedlimitAbs   string      `json:"speedlimit_abs"`
	Slots           []Slot      `json:"slots"`
	Start           int64       `json:"start"`
	Finish          int64       `json:"finish"`
	Limit           int64       `json:"limit"`
	Noofslots       int64       `json:"noofslots"`
	NoofslotsTotal  int64       `json:"noofslots_total"`
	PausedAll       bool        `json:"paused_all"`
	HaveQuota       bool        `json:"have_quota"`
	Paused          bool        `json:"paused"`
}

type Slot struct {
	DirectUnpack *string  `json:"direct_unpack"`
	Mbmissing    string   `json:"mbmissing"`
	AvgAge       string   `json:"avg_age"`
	Sizeleft     string   `json:"sizeleft"`
	Filename     string   `json:"filename"`
	Size         string   `json:"size"`
	MB           string   `json:"mb"`
	Mbleft       string   `json:"mbleft"`
	Status       string   `json:"status"`
	Unpackopts   string   `json:"unpackopts"`
	Password     string   `json:"password"`
	Script       string   `json:"script"`
	NzoID        string   `json:"nzo_id"`
	Percentage   string   `json:"percentage"`
	Cat          string   `json:"cat"`
	Timeleft     string   `json:"timeleft"`
	Labels       []string `json:"labels"`
	Priority     int      `json:"priority"`
	Index        int64    `json:"index"`
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

	ids := make([]string, 0)
	for _, s := range response.Queue.Slots {
		ids = append(ids, s.NzoID)
	}

	historyResponse, err := c.history(ctx, ids...)
	if err != nil {
		return nil, err
	}

	return queueToStatus(response.Queue, historyResponse.History)
}

type HistoryResponse struct {
	History History `json:"history"`
}

// History represents the root history structure
type History struct {
	DaySize           string        `json:"day_size"`
	WeekSize          string        `json:"week_size"`
	MonthSize         string        `json:"month_size"`
	TotalSize         string        `json:"total_size"`
	Slots             []HistorySlot `json:"slots"`
	NoOfSlots         int           `json:"noofslots"`
	PPslots           int           `json:"ppslots"`
	LastHistoryUpdate int64         `json:"last_history_update"`
}

// HistorySlot represents an individual slot in the history
type HistorySlot struct {
	Meta         interface{} `json:"meta"`
	Status       string      `json:"status"`
	Size         string      `json:"size"`
	ScriptLine   string      `json:"script_line"`
	URLInfo      string      `json:"url_info"`
	MD5Sum       string      `json:"md5sum"`
	Category     string      `json:"category"`
	PP           string      `json:"pp"`
	URL          string      `json:"url"`
	Script       string      `json:"script"`
	NZBName      string      `json:"nzb_name"`
	Name         string      `json:"name"`
	NzoID        string      `json:"nzo_id"`
	Path         string      `json:"path"`
	ActionLine   string      `json:"action_line"`
	FailMessage  string      `json:"fail_message"`
	DuplicateKey string      `json:"duplicate_key"`
	Storage      string      `json:"storage"`
	Password     string      `json:"password"`
	Report       string      `json:"report"`
	StageLog     []StageLog  `json:"stage_log"`
	Downloaded   int64       `json:"downloaded"`
	PostprocTime int         `json:"postproc_time"`
	DownloadTime int         `json:"download_time"`
	Retry        int         `json:"retry"`
	Completed    int64       `json:"completed"`
	Bytes        int64       `json:"bytes"`
	HasRating    bool        `json:"has_rating"`
	Archive      bool        `json:"archive"`
	Loaded       bool        `json:"loaded"`
}

// StageLog represents a single stage log entry
type StageLog struct {
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

func (c *SabnzbdClient) history(ctx context.Context, ids ...string) (HistoryResponse, error) {
	url := url.URL{
		Host:   c.host,
		Scheme: c.scheme,
		Path:   "/sabnzbd/api",
	}

	q := url.Query()
	q.Set("mode", "history")
	if len(ids) > 0 {
		q.Set("nzo_id", strings.Join(ids, ","))
	}

	url.RawQuery = q.Encode()

	var history HistoryResponse
	b, err := c.do(ctx, &url)
	if err != nil {
		return history, err
	}

	err = json.Unmarshal(b, &history)
	return history, err
}

func queueToStatus(queue Queue, history History) ([]Status, error) {
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

		var path string
		for _, h := range history.Slots {
			if h.NzoID == s.NzoID {
				path = h.Storage
			}
		}

		stats[i] = Status{
			ID:       s.NzoID,
			Name:     s.Filename,
			Progress: p,
			Size:     int64(size),
			Speed:    int64(speed),
			FilePath: []string{path},
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
