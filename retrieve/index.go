package retrieve

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/seankhliao/gomodstats"
)

// Index only queries the index to discover modules/versions
func (c *Client) Index() (*gomodstats.Store, error) {
	var since string
	var cur, total int
	var mods map[string][]gomodstats.Module
	for cur == 2000 {
		cur = 0
		u := c.IndexUrl
		if since != "" {
			u += "?since=" + since
		}
		res, err := http.Get(u)
		if err != nil {
			return nil, fmt.Errorf("Index get %s: %w", u, err)
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("Index get %s: status %d %s", u, res.StatusCode, res.Status)
		}
		defer res.Body.Close()
		d := json.NewDecoder(res.Body)
		var ir indexRecord
		for d.More() {
			err = d.Decode(&ir)
			if err != nil {
				return nil, fmt.Errorf("Index decode: %w", err)
			}
			mods[ir.Path] = append(mods[ir.Path], gomodstats.Module{
				Name:    ir.Path,
				Version: ir.Version,
				Indexed: ir.Timestamp,
			})
			cur, total, since = cur+1, total+1, ir.Timestamp
		}
	}
	return &gomodstats.Store{
		Mods: mods,
	}, nil
}

// indexRecord is a record on index.golang.org
type indexRecord struct {
	Path      string
	Version   string
	Timestamp string
}
