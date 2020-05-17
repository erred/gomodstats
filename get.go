package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.seankhliao.com/gomodstats/v2/pb"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/proto"
)

func Modules(pbi *pb.Index) {
	var modt, modv, done, gone, errc int64

	mods := make(map[string][]string)
	for _, ir := range pbi.Records {
		mods[ir.Path] = append(mods[ir.Path], ir.Version)
		modv++
	}
	modt = int64(len(mods))

	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)
	for i := 0; i < limit; i++ {
		sem <- struct{}{}
	}

	defer fmt.Printf("progress done=%v gone=%v err=%v mods=%v modv=%v\n", atomic.LoadInt64(&done), atomic.LoadInt64(&gone), atomic.LoadInt64(&errc), modt, modv)
	go func() {
		for range time.NewTicker(15 * time.Second).C {
			fmt.Printf("progress done=%v gone=%v err=%v mods=%v modv=%v\n", atomic.LoadInt64(&done), atomic.LoadInt64(&gone), atomic.LoadInt64(&errc), modt, modv)
		}
	}()

	for m, vers := range mods {
		sort.Slice(vers, func(i, j int) bool {
			return semver.Compare(vers[i], vers[j]) == -1
		})

		for _, v := range vers {
			wg.Add(1)
			<-sem
			go func(m, v string) {
				defer func() {
					wg.Done()
					sem <- struct{}{}
				}()
				err := getMod(m, v)
				if err != nil && strings.Contains(err.Error(), "410 Gone") {
					atomic.AddInt64(&gone, 1)
				} else if err != nil {
					log.Printf("mod %v", err)
					atomic.AddInt64(&errc, 1)
					return
				} else {
					atomic.AddInt64(&done, 1)
				}
			}(m, v)
		}
	}
	wg.Wait()
}

func getMod(m, v string) error {
	bufi := pool.Get()
	buf, ok := bufi.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("buffer assert failed, type=%T", bufi)
	}
	defer func() {
		buf.Reset()
		pool.Put(buf)
	}()

	res, err := http.Get(fmt.Sprintf("%s/%s/@v/%s.mod", proxyURL, m, v))
	if err != nil {
		return fmt.Errorf("modfile get %s %s: %w", m, v, err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("modfile get %s %s: %d %s", m, v, res.StatusCode, res.Status)
	}
	defer res.Body.Close()
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		return fmt.Errorf("modfile read %s %s: %w", m, v, err)
	}
	mf, err := modfile.Parse(fmt.Sprintf("%s@%s", m, v), buf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("modfile parse %s %s: %w", m, v, err)
	}
	pbm := pb.ModuleVersion{
		Version: v,
	}
	if mf.Go != nil {
		pbm.Go = mf.Go.Version
	}
	for _, r := range mf.Require {
		pbm.Requires = append(pbm.Requires, &pb.Require{
			Version: &pb.Version{
				Module:  r.Mod.Path,
				Version: r.Mod.Version,
			},
			Indirect: r.Indirect,
		})
	}
	for _, r := range mf.Exclude {
		pbm.Excludes = append(pbm.Excludes, &pb.Version{
			Module:  r.Mod.Path,
			Version: r.Mod.Version,
		})
	}
	for _, r := range mf.Replace {
		pbm.Replaces = append(pbm.Replaces, &pb.Replace{
			Old: &pb.Version{
				Module:  r.Old.Path,
				Version: r.Old.Version,
			},
			New: &pb.Version{
				Module:  r.New.Path,
				Version: r.New.Version,
			},
		})
	}

	buf.Reset()
	res, err = http.Get(fmt.Sprintf("%s/%s/@v/%s.zip", proxyURL, m, v))
	if err != nil {
		return fmt.Errorf("module get %s %s: %w", m, v, err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("module status %s %s: %d %s", m, v, res.StatusCode, res.Status)
	}
	defer res.Body.Close()
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		return fmt.Errorf("module read %s %s: %w", m, v, err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return fmt.Errorf("module unzip %s %s: %w", m, v, err)
	}
	fset := token.NewFileSet()

	pbm.Tokens = make(map[string]int64, 90)
	pbm.Idents = make(map[string]int64, 1000)

	bufi = pool.Get()
	buf2, ok := bufi.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("buffer assert failed, type=%T", bufi)
	}
	defer func() {
		buf2.Reset()
		pool.Put(buf2)
	}()

	for _, zf := range r.File {
		if filepath.Ext(zf.Name) != ".go" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("module open %s %s %s: %w", m, v, zf.Name, err)
		}
		buf2.Reset()
		_, err = buf2.ReadFrom(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("module read %s %s %s: %w", m, v, zf.Name, err)
		}
		file := fset.AddFile(zf.Name, fset.Base(), buf2.Len())
		var s scanner.Scanner
		s.Init(file, buf2.Bytes(), nil, scanner.ScanComments)
		for {
			_, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			} else if tok == token.IDENT {
				pbm.Idents[lit]++
			}
			pbm.Tokens[tok.String()]++
		}
	}

	b, err := proto.Marshal(&pbm)
	if err != nil {
		return fmt.Errorf("module marshal %s %s: %w", m, v, err)
	}
	f := fmt.Sprintf("%s/%s@%s.pb", "mods", strings.ReplaceAll(m, "/", "--"), v)
	err = ioutil.WriteFile(f, b, 0o644)
	if err != nil {

		return fmt.Errorf("mod write %s: %w", f, err)
	}
	return nil
}

func Index() (*pb.Index, error) {
	var pbi pb.Index
	b, err := ioutil.ReadFile(chkptIndex)
	if err == nil {
		err = proto.Unmarshal(b, &pbi)
		if err == nil {
			return &pbi, nil
		}
		log.Println("Index unmarshal checkpoint err:", err)
	}
	log.Println("Index read checkpoint err:", err)
	// log error?

	ts, prev := "", 2000
	for prev == 2000 {
		u := indexURL
		if ts != "" {
			u += "?since=" + ts
		}
		res, err := http.Get(u)
		if err != nil {
			return nil, fmt.Errorf("Index get: %w", err)
		} else if res.StatusCode != 200 {
			return nil, fmt.Errorf("Index status: %d %s", res.StatusCode, res.Status)
		}
		prev = 0
		d := json.NewDecoder(res.Body)
		for d.More() {
			var ir pb.IndexRecord
			err = d.Decode(&ir)
			if err != nil {
				return nil, fmt.Errorf("Index decode: %w", err)
			}
			pbi.Records = append(pbi.Records, &ir)
			prev++
			ts = ir.Timestamp
		}
	}

	b, err = proto.Marshal(&pbi)
	if err != nil {
		return nil, fmt.Errorf("Index marshal: %w", err)
	}
	err = ioutil.WriteFile(chkptIndex, b, 0644)
	if err != nil {
		return nil, fmt.Errorf("Index write: %w", err)
	}
	return &pbi, nil
}
