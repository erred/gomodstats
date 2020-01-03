package retrieve

import (
	"net/http"
	"time"
)

// DefaultClient is a client that uses the default
// golang.org index/proxy
var DefaultClient = &Client{
	IndexUrl: "https://index.golang.org/index",
	ProxyUrl: "https://proxy.golang.org",
	Parallel: 200,
	Timeout:  20 * time.Second,
	http: &http.Client{
		Timeout: 20 * time.Second,
	},
}

// Client holds info needed to identify servers
type Client struct {
	IndexUrl string
	ProxyUrl string
	Parallel int
	Timeout  time.Duration
	http     *http.Client
}
