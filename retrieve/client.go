package retrieve

// DefaultClient is a client that uses the default
// golang.org index/proxy
var DefaultClient = &Client{
	IndexUrl: "https://index.golang.org/index",
	ProxyUrl: "https://proxy.golang.org",
}

// Client holds info needed to identify servers
type Client struct {
	IndexUrl string
	ProxyUrl string
}
