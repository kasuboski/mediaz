package prowlarr

import (
	"net/http"
)

func SetRequestAPIKey(apiKey string, req *http.Request) {
	q := req.URL.Query()
	q.Set("apikey", apiKey)
	req.URL.RawQuery = q.Encode()
}
