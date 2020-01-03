package retrieve

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/seankhliao/gomodstats"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/vcs"
)

// ProxyMeta fills in additional metadata from quering the module proxy
// such as more versions,
// modfile,
// vcs
func (c *Client) ProxyMeta(store *gomodstats.Store) (*gomodstats.Store, []error) {
	in, out := make(chan modVers, 100), make(chan modVers, 100)
	errc := make(chan error, 100)
	wg, collect := &sync.WaitGroup{}, &sync.WaitGroup{}

	go func() {
		for m, vers := range store.Mods {
			in <- modVers{m, vers}
		}
		close(in)
	}()

	wg.Add(100)
	for i := 0; i < 100; i++ {
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
		for v, m := range vers {
			m, err := c.getModFile(v, m)
			if err != nil {
				errc <- err
				continue
			}
			nv = append(nv, m)
		}
		out <- modVers{mv.name, nv}
	}
}

func (c *Client) getModuleVersions(mv modVers, vers map[string]gomodstats.Module) error {
	u := fmt.Sprintf("%s/%s/@v/list", c.ProxyUrl, mv.name)
	res, err := http.Get(u)
	if err != nil {
		return fmt.Errorf("getModuleVersions list %s: %w", mv.name, err)
	}
	if res.StatusCode != 200 {
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
			}
		}
	}
	return nil
}

func (c *Client) getModFile(v string, m gomodstats.Module) (gomodstats.Module, error) {
	u := fmt.Sprintf("%s/%s/@v/%s.mod", c.ProxyUrl, m.Name, v)
	res, err := http.Get(u)
	if err != nil {
		return m, fmt.Errorf("getModFile modfile %s %s: %w", m.Name, v, err)
	}
	if res.StatusCode != 200 {
		return m, fmt.Errorf("getModFile modfile %s %s: %d %s", m.Name, v, res.StatusCode, res.Status)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return m, fmt.Errorf("getMpdFile read %s %s: %w", m.Name, v, err)
	}
	m.ModFile, err = modfile.Parse(m.Name+"/@v/"+v+".mod", b, nil)
	if err != nil {
		return m, fmt.Errorf("getModFile parse %s %s: %w", m.Name, v, err)
	}
	m.RepoRoot, err = vcs.RepoRootForImportPath(m.Name, false)
	if err != nil {
		return m, fmt.Errorf("getModFile repo root %s %s: %w", m.Name, v, err)
	}
	return m, nil
}

type modVers struct {
	name string
	mods []gomodstats.Module
}
