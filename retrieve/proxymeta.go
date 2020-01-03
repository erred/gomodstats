package retrieve

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/seankhliao/gomodstats"
	"golang.org/x/mod/modfile"
)

// ProxyMeta fills in additional metadata from quering the module proxy
// such as more versions,
// modfile,
// vcs (x)
func (c *Client) ProxyMeta(store *gomodstats.Store) (*gomodstats.Store, []error) {
	in, out := make(chan modVers, c.Parallel), make(chan modVers, c.Parallel)
	errc := make(chan error, c.Parallel)
	wg, collect := &sync.WaitGroup{}, &sync.WaitGroup{}

	go func() {
		var c int
		for m, vers := range store.Mods {
			c++
			in <- modVers{m, vers}
			if c%1000 == 0 {
				log.Info().Int("current", c).Int("total", len(store.Mods)).Msg("progress")
			}
		}
		close(in)
	}()

	wg.Add(c.Parallel)
	for i := 0; i < c.Parallel; i++ {
		go c.getModuleMeta(in, out, errc, wg)
	}
	go func() {
		wg.Wait()
		close(out)
		close(errc)
	}()

	mods := make(map[string][]gomodstats.Module, len(store.Mods))
	var errs []error
	collect.Add(2)
	go func() {
		defer collect.Done()
		for modVers := range out {
			mods[modVers.name] = modVers.mods
		}
	}()
	go func() {
		defer collect.Done()
		for err := range errc {
			errs = append(errs, err)
		}
	}()

	collect.Wait()
	return &gomodstats.Store{
		Mods: mods,
	}, errs
}

func (c *Client) getModuleMeta(in, out chan modVers, errc chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	for mv := range in {
		vers := make(map[string]gomodstats.Module, len(mv.mods))
		for _, m := range mv.mods {
			vers[m.Version] = m
		}

		// get version list
		err := c.getModuleVersions(mv, vers)
		if err != nil {
			errc <- err
			continue
		}

		// get modfile / vcs
		nv := make([]gomodstats.Module, 0, len(vers))
		nvc := make(chan gomodstats.Module, len(vers))
		vwg := &sync.WaitGroup{}
		vwg.Add(len(vers))
		for v, m := range vers {
			go func(v string, m gomodstats.Module) {
				defer vwg.Done()
				m, err := c.getModFile(v, m)
				if err != nil {
					errc <- err
					return
				}
				nvc <- m
			}(v, m)
		}
		go func() {
			vwg.Wait()
			close(nvc)
		}()
		for m := range nvc {
			nv = append(nv, m)
		}
		out <- modVers{mv.name, nv}
	}
}

func (c *Client) getModuleVersions(mv modVers, vers map[string]gomodstats.Module) error {
	u := fmt.Sprintf("%s/%s/@v/list", c.ProxyUrl, mv.name)
	res, err := c.http.Get(u)
	if err != nil {
		return fmt.Errorf("getModuleVersions list %s: %w", mv.name, err)
	} else if res.StatusCode == 410 {
		for k, v := range vers {
			v.Proxied = false
			vers[k] = v
		}
		return nil
	} else if res.StatusCode != 200 {
		return fmt.Errorf("getModuleVersions list %s: %d %s", mv.name, res.StatusCode, res.Status)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("getModuleVersions read %s: %w", mv.name, err)
	}
	listvers := bytes.Fields(b)
	for _, v := range listvers {
		if _, ok := vers[string(v)]; !ok {
			vers[string(v)] = gomodstats.Module{
				Name:    mv.name,
				Version: string(v),
				Proxied: true,
			}
		}
	}
	return nil
}

func (c *Client) getModFile(v string, m gomodstats.Module) (gomodstats.Module, error) {
	u := fmt.Sprintf("%s/%s/@v/%s.mod", c.ProxyUrl, m.Name, v)
	res, err := c.http.Get(u)
	if err != nil {
		return m, fmt.Errorf("getModFile modfile %s %s: %w", m.Name, v, err)
	} else if res.StatusCode == 410 {
		m.Proxied = false
		return m, nil
	} else if res.StatusCode != 200 {
		return m, fmt.Errorf("getModFile modfile %s %s: %d %s", m.Name, v, res.StatusCode, res.Status)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return m, fmt.Errorf("getMpdFile read %s %s: %w", m.Name, v, err)
	}
	m.ModFile, err = modfile.Parse(m.Name+"/@v/"+v+".mod", b, nil)
	if err != nil {
		m.ModFileErr = err
		log.Warn().Str("mod", m.Name).Str("ver", v).Err(err).Msg("parse modfile")
	}
	// rr := make(chan *vcs.RepoRoot)
	// errs := make(chan error)
	// ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	// defer cancel()
	// go func() {
	// 	repoRoot, err := vcs.RepoRootForImportPath(m.Name, false)
	// 	if err != nil {
	// 		errs <- fmt.Errorf("getModFile repo root %s %s: %w", m.Name, v, err)
	// 		return
	// 	}
	// 	rr <- repoRoot
	// }()
	// select {
	// case m.RepoRoot = <-rr:
	// 	// noop
	// case err := <-errs:
	// 	log.Error().Str("mod", m.Name).Str("ver", v).Err(err).Msg("repoRoot")
	// case <-ctx.Done():
	// 	log.Error().Str("mod", m.Name).Str("ver", v).Err(errors.New("timeout")).Msg("repoRoot")
	// }
	return m, nil
}

type modVers struct {
	name string
	mods []gomodstats.Module
}
