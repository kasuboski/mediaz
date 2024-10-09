package transmission

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/kasuboski/mediaz/pkg/download"
)

type Client struct {
	http    *http.Client
	scheme  string
	host    string
	mutex   *sync.Mutex
	session string
}

type TransmissionRequest struct {
	Arguments any           `json:"arguments"`
	Tag       *int          `json:"tag,omitempty"`
	Method    torrentMethod `json:"method"`
}

type torrentMethod string

const (
	AddTorrentMethod torrentMethod = "torrent-add"
	GetTorrentMethod torrentMethod = "torrent-get"
)

func NewClient(http *http.Client, scheme, host string, port int) download.Client {
	return &Client{
		http:    http,
		scheme:  scheme,
		host:    fmt.Sprintf("%s:%d", host, port),
		mutex:   new(sync.Mutex),
		session: "",
	}
}

type Torrent struct {
	Name                string        `json:"name"`
	HashString          string        `json:"hashString"`
	DownloadDir         string        `json:"downloadDir"`
	Pieces              string        `json:"pieces"`
	Files               []File        `json:"files"`
	TrackerStats        []TrackerStat `json:"trackerStats"`
	Trackers            []Tracker     `json:"trackers"`
	Peers               []Peer        `json:"peers"`
	Wanted              []int         `json:"wanted"`
	Priorities          []int         `json:"priorities"`
	FileStats           []FileStat    `json:"fileStats"`
	SizeWhenDone        int64         `json:"sizeWhenDone"`
	DesiredAvailable    int64         `json:"desiredAvailable"`
	ETA                 int64         `json:"eta"`
	PeersConnected      int           `json:"peersConnected"`
	PeersGettingFromUs  int           `json:"peersGettingFromUs"`
	PeersSendingToUs    int           `json:"peersSendingToUs"`
	PercentDone         float64       `json:"percentDone"`
	ID                  int           `json:"id"`
	TotalSize           int64         `json:"totalSize"`
	LeftUntilDone       int64         `json:"leftUntilDone"`
	RecheckProgress     float64       `json:"recheckProgress"`
	ActivityDate        int64         `json:"activityDate"`
	CorruptEver         int64         `json:"corruptEver"`
	RateUpload          int64         `json:"rateUpload"`
	RateDownload        int64         `json:"rateDownload"`
	SeedRatioLimit      float64       `json:"seedRatioLimit"`
	PieceCount          int           `json:"pieceCount"`
	PieceSize           int64         `json:"pieceSize"`
	UploadLimit         int           `json:"uploadLimit"`
	SeedRatioMode       int           `json:"seedRatioMode"`
	DownloadLimit       int           `json:"downloadLimit"`
	WebseedsSendingToUs int           `json:"webseedsSendingToUs"`
	DoneDate            int64         `json:"doneDate"`
	AddedDate           int64         `json:"addedDate"`
	Status              int           `json:"status"`
	UploadRatio         float64       `json:"uploadRatio"`
	DownloadLimited     bool          `json:"downloadLimited"`
	UploadLimited       bool          `json:"uploadLimited"`
}

type File struct {
	Name           string `json:"name"`
	Length         int64  `json:"length"`
	BytesCompleted int64  `json:"bytesCompleted"`
}

type FileStat struct {
	BytesCompleted int64 `json:"bytesCompleted"`
	Wanted         bool  `json:"wanted"`
	Priority       int   `json:"priority"`
}

type Peer struct {
	ClientName         string  `json:"clientName"`
	FlagStr            string  `json:"flagStr"`
	Address            string  `json:"address"`
	Port               int     `json:"port"`
	RateToPeer         int64   `json:"rateToPeer"`
	RateToClient       int64   `json:"rateToClient"`
	Progress           float64 `json:"progress"`
	IsEncrypted        bool    `json:"isEncrypted"`
	IsUTP              bool    `json:"isUTP"`
	PeerIsChoked       bool    `json:"peerIsChoked"`
	PeerIsInterested   bool    `json:"peerIsInterested"`
	IsIncoming         bool    `json:"isIncoming"`
	IsDownloadingFrom  bool    `json:"isDownloadingFrom"`
	ClientIsInterested bool    `json:"clientIsInterested"`
	ClientIsChoked     bool    `json:"clientIsChoked"`
}

type Tracker struct {
	Announce string `json:"announce"`
	Scrape   string `json:"scrape"`
	ID       int    `json:"id"`
	Tier     int    `json:"tier"`
}

type TrackerStat struct {
	Host                  string `json:"host"`
	LastScrapeResult      string `json:"lastScrapeResult"`
	LastAnnounceResult    string `json:"lastAnnounceResult"`
	ScrapeState           int    `json:"scrapeState"`
	LeecherCount          int    `json:"leecherCount"`
	LastAnnouncePeerCount int    `json:"lastAnnouncePeerCount"`
	SeederCount           int    `json:"seederCount"`
	LastAnnounceStartTime int64  `json:"lastAnnounceStartTime"`
	NextScrapeTime        int64  `json:"nextScrapeTime"`
	LastAnnounceTime      int64  `json:"lastAnnounceTime"`
	AnnounceState         int    `json:"announceState"`
	DownloadCount         int    `json:"downloadCount"`
	LastScrapeStartTime   int64  `json:"lastScrapeStartTime"`
	NextAnnounceTime      int64  `json:"nextAnnounceTime"`
	LastScrapeTime        int64  `json:"lastScrapeTime"`
	LastAnnounceTimedOut  bool   `json:"lastAnnounceTimedOut"`
	LastScrapeTimedOut    bool   `json:"lastScrapeTimedOut"`
	LastScrapeSucceeded   bool   `json:"lastScrapeSucceeded"`
	LastAnnounceSucceeded bool   `json:"lastAnnounceSucceeded"`
	HasScraped            bool   `json:"hasScraped"`
	HasAnnounced          bool   `json:"hasAnnounced"`
}

type ListTorrentsResponse struct {
	Result    string      `json:"result"`
	Arguments TorrentList `json:"arguments"`
}

func (r ListTorrentsResponse) ToTorrents() []download.Status {
	var torrents []download.Status
	for _, r := range r.Arguments.Torrents {
		torrents = append(torrents, download.Status{
			ID:   fmt.Sprintf("%d", r.ID),
			Name: r.Name,
		})
	}

	return torrents
}

type TorrentList struct {
	Torrents []Torrent `json:"torrents"`
}

var (
	torrentFields = []string{
		"activityDate",
		"addedDate",
		"bandwidthPriority",
		"comment",
		"corruptEver",
		"creator",
		"dateCreated",
		"desiredAvailable",
		"downloadDir",
		"downloadedEver",
		"error",
		"errorString",
		"eta",
		"etaIdle",
		"files",
		"fileStats",
		"hashString",
		"honorsSessionLimits",
		"id",
		"isFinished",
		"isSeed",
		"lastAnnounceTime",
		"lastScrapeTime",
		"magnetLink",
		"manualAnnounceTime",
		"metadataPercentComplete",
		"name",
		"peers",
		"peersConnected",
		"peersGettingFromUs",
		"peersSendingToUs",
		"percentDone",
		"priorities",
		"rateDownload",
		"rateUpload",
		"recheckProgress",
		"status",
		"totalSize",
		"torrentFile",
		"uploadedEver",
		"uploadLimit",
		"uploadRatio",
		"wanted",
		"webseeds",
		"webseedsSendingToUs",
	}
)

// List fetches all torrents
func (c *Client) List(ctx context.Context) ([]download.Status, error) {
	arguments := make(map[string]any)
	arguments["fields"] = torrentFields

	request := &TransmissionRequest{
		Method:    GetTorrentMethod,
		Arguments: arguments,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	url := url.URL{
		Host:   c.host,
		Scheme: c.scheme,
		Path:   "/transmission/rpc",
	}

	b, err = c.do(ctx, &url, b)
	if err != nil {
		return nil, err
	}

	var response ListTorrentsResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return nil, err
	}

	return response.ToTorrents(), nil
}

// Get fetches a torrent given an id
func (c *Client) Get(ctx context.Context, request download.GetRequest) (download.Status, error) {
	var status download.Status
	arguments := make(map[string]any)
	arguments["fields"] = torrentFields
	arguments["ids"] = []int{request.ID}

	transmissionRequest := &TransmissionRequest{
		Method:    GetTorrentMethod,
		Arguments: arguments,
	}

	b, err := json.Marshal(transmissionRequest)
	if err != nil {
		return status, err
	}

	url := url.URL{
		Host:   c.host,
		Scheme: c.scheme,
		Path:   "/transmission/rpc",
	}

	b, err = c.do(ctx, &url, b)
	if err != nil {
		return status, err
	}

	var response ListTorrentsResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return status, err
	}

	torrents := response.ToTorrents()
	if len(torrents) == 0 {
		return status, fmt.Errorf("no torrent found for %d", request.ID)
	}

	return torrents[0], nil
}

// Add creates a new torrent
func (c *Client) Add(ctx context.Context, request download.AddRequest) (download.Status, error) {
	var status download.Status
	arguments := make(map[string]any)
	arguments["fields"] = torrentFields

	transmissionRequest := &TransmissionRequest{
		Method:    AddTorrentMethod,
		Arguments: arguments,
	}

	b, err := json.Marshal(transmissionRequest)
	if err != nil {
		return status, err
	}

	url := url.URL{
		Host:   c.host,
		Scheme: c.scheme,
		Path:   "/transmission/rpc",
	}

	b, err = c.do(ctx, &url, b)
	if err != nil {
		return status, fmt.Errorf("error during request: %v", err)
	}

	var response ListTorrentsResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return status, err
	}

	torrents := response.ToTorrents()
	if len(torrents) == 0 {
		return status, errors.New("no torrents found")
	}

	return torrents[0], nil
}

const (
	sessionHeader = "x-transmission-session-id"
)

func (c *Client) do(ctx context.Context, url *url.URL, body []byte, retry ...bool) ([]byte, error) {
	if c.http == nil {
		return nil, errors.New("http client is nil")
	}

	if url == nil {
		return nil, errors.New("url is nil")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(sessionHeader, c.getSessionID())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	// need to get a new session id from the response if 409
	case http.StatusConflict:
		// prevent infinitely attempting to get a new session id if we got a session previously in this same request attempt
		if len(retry) != 0 && retry[0] {
			return nil, errors.New("session id is invalid after retry")
		}

		session := resp.Header.Get(sessionHeader)
		if session == "" {
			return nil, errors.New("session id is empty")
		}

		// make the request again with new session
		c.setSessionID(session)
		return c.do(ctx, url, body, true)

	case http.StatusOK:
		return io.ReadAll(resp.Body)

	default:
		return nil, fmt.Errorf("unexpected status code: %v", resp.Status)
	}
}

func (c *Client) setSessionID(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.session = id
}

func (c *Client) getSessionID() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.session
}
