package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/kasuboski/mediaz/pkg/logger"
	"go.uber.org/zap"
)

type TransmissionClient struct {
	http        HTTPClient
	scheme      string
	host        string
	mutex       *sync.Mutex
	session     string
	mountPrefix string
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

func NewTransmissionClient(http HTTPClient, scheme, host, mountPrefix string, port int) DownloadClient {
	if port != 0 {
		host = fmt.Sprintf("%s:%d", host, port)
	}

	return &TransmissionClient{
		http:        http,
		scheme:      scheme,
		host:        host,
		mutex:       new(sync.Mutex),
		session:     "",
		mountPrefix: mountPrefix,
	}
}

type TransmissionTorrent struct {
	Name                string                    `json:"name"`
	HashString          string                    `json:"hashString"`
	DownloadDir         string                    `json:"downloadDir"`
	Pieces              string                    `json:"pieces"`
	Files               []TransmissionFile        `json:"files"`
	TrackerStats        []TransmissionTrackerStat `json:"trackerStats"`
	Trackers            []TransmissionTracker     `json:"trackers"`
	Peers               []TransmissionPeer        `json:"peers"`
	Wanted              []int                     `json:"wanted"`
	Priorities          []int                     `json:"priorities"`
	FileStats           []TransmissionFileStat    `json:"fileStats"`
	SizeWhenDone        int64                     `json:"sizeWhenDone"`
	DesiredAvailable    int64                     `json:"desiredAvailable"`
	ETA                 int64                     `json:"eta"`
	PeersConnected      int                       `json:"peersConnected"`
	PeersGettingFromUs  int                       `json:"peersGettingFromUs"`
	PeersSendingToUs    int                       `json:"peersSendingToUs"`
	PercentDone         float64                   `json:"percentDone"`
	ID                  int                       `json:"id"`
	TotalSize           int64                     `json:"totalSize"`
	LeftUntilDone       int64                     `json:"leftUntilDone"`
	RecheckProgress     float64                   `json:"recheckProgress"`
	ActivityDate        int64                     `json:"activityDate"`
	CorruptEver         int64                     `json:"corruptEver"`
	RateUpload          int64                     `json:"rateUpload"`
	RateDownload        int64                     `json:"rateDownload"`
	SeedRatioLimit      float64                   `json:"seedRatioLimit"`
	PieceCount          int                       `json:"pieceCount"`
	PieceSize           int64                     `json:"pieceSize"`
	UploadLimit         int                       `json:"uploadLimit"`
	SeedRatioMode       int                       `json:"seedRatioMode"`
	DownloadLimit       int                       `json:"downloadLimit"`
	WebseedsSendingToUs int                       `json:"webseedsSendingToUs"`
	DoneDate            int64                     `json:"doneDate"`
	AddedDate           int64                     `json:"addedDate"`
	Status              float64                   `json:"status"`
	UploadRatio         float64                   `json:"uploadRatio"`
	DownloadLimited     bool                      `json:"downloadLimited"`
	UploadLimited       bool                      `json:"uploadLimited"`
}

func (t *TransmissionTorrent) ToStatus(mountPrefix string) Status {
	var paths []string
	for _, f := range t.Files {
		paths = append(paths, filepath.Join(mountPrefix, t.DownloadDir, f.Name))
	}

	log.Printf("%+v", t)
	s := Status{
		ID:        fmt.Sprintf("%d", t.ID),
		Name:      t.Name,
		Size:      t.TotalSize >> 20, // bytes to mb
		Progress:  t.PercentDone,
		Speed:     t.RateDownload >> 20, // bytes/s to mb/s
		FilePaths: paths,
		Done:      t.Status > 4 || t.PercentDone == 100.0, // 4 = downloading, 5 = queue'd to seed, 6 = seeding
	}

	return s
}

type TransmissionFile struct {
	Name           string `json:"name"`
	Length         int64  `json:"length"`
	BytesCompleted int64  `json:"bytesCompleted"`
}

type TransmissionFileStat struct {
	BytesCompleted int64 `json:"bytesCompleted"`
	Wanted         bool  `json:"wanted"`
	Priority       int   `json:"priority"`
}

type TransmissionPeer struct {
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

type TransmissionTracker struct {
	Announce string `json:"announce"`
	Scrape   string `json:"scrape"`
	ID       int    `json:"id"`
	Tier     int    `json:"tier"`
}

type TransmissionTrackerStat struct {
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

type TransmissionListTorrentsResponse struct {
	Result    string      `json:"result"`
	Arguments TorrentList `json:"arguments"`
}

func (r TransmissionListTorrentsResponse) ToTorrents(mountPrefix string) []Status {
	var torrents []Status
	for _, r := range r.Arguments.Torrents {
		torrents = append(torrents, r.ToStatus(mountPrefix))
	}

	return torrents
}

type TorrentList struct {
	Torrents []TransmissionTorrent `json:"torrents"`
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

type AddTorrentPayload struct {
	DownloadDir string   `json:"download-dir"`
	Filename    string   `json:"filename"`
	MetaInfo    string   `json:"metainfo"`
	Labels      []string `json:"labels"`
}

// Get fetches a torrent given an id
func (c *TransmissionClient) Get(ctx context.Context, request GetRequest) (Status, error) {
	var status Status

	id, err := strconv.Atoi(request.ID)
	if err != nil {
		return status, err
	}

	arguments := make(map[string]any)
	arguments["fields"] = torrentFields
	arguments["ids"] = []int{id}

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

	var response TransmissionListTorrentsResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return status, err
	}

	if response.Result != "success" {
		return status, fmt.Errorf("unexpected result: %v", response.Result)
	}

	torrents := response.ToTorrents(c.mountPrefix)
	if len(torrents) == 0 {
		return status, fmt.Errorf("no torrent found for %s", request.ID)
	}

	return torrents[0], nil
}

// List fetches all torrents
func (c *TransmissionClient) List(ctx context.Context) ([]Status, error) {
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

	var response TransmissionListTorrentsResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("unexpected result: %v", response.Result)
	}

	return response.ToTorrents(c.mountPrefix), nil
}

// AddTorrentResponse represents a response from a torrent-add rpc call
type AddTorrentResponse struct {
	Result    string                      `json:"result"`
	Arguments AddTorrentResponseArguments `json:"arguments"`
}

// AddTorrentResponseArguments contains the details about the added torrent
type AddTorrentResponseArguments struct {
	TorrentAdded AddedTorrent `json:"torrent-added"`
}

// AddedTorrent represents the details of the added torrent
type AddedTorrent struct {
	HashString string `json:"hashString"`
	Name       string `json:"name"`
	ID         int    `json:"id"`
}

// Add creates a new torrent
func (c *TransmissionClient) Add(ctx context.Context, request AddRequest) (Status, error) {
	var status Status

	log := logger.FromCtx(ctx)
	uri, err := request.Release.GUID.Get()
	if err != nil {
		log.Debug("failed to get uri from release", zap.Error(err))
		return status, err
	}

	arguments := AddTorrentPayload{
		DownloadDir: "/downloads/transmission",
		Filename:    uri,
	}

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
		return status, err
	}

	var response AddTorrentResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		return status, err
	}

	if response.Result != "success" {
		return status, fmt.Errorf("unexpected result: %v", response.Result)

	}

	return c.Get(ctx, GetRequest{ID: strconv.Itoa(response.Arguments.TorrentAdded.ID)})
}

const (
	sessionHeader = "x-transmission-session-id"
)

func (c *TransmissionClient) do(ctx context.Context, url *url.URL, body []byte, retry ...bool) ([]byte, error) {
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

func (c *TransmissionClient) setSessionID(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.session = id
}

func (c *TransmissionClient) getSessionID() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.session
}
