package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "net/http/pprof"

	"go.seankhliao.com/gomodstats/v2/pb"
	"golang.org/x/mod/semver"
)

const (
	indexURL = "https://index.golang.org/index"
	proxyURL = "https://proxy.golang.org"

	chkptIndex = "index.pb"

	limit = 200
)

var (
	pool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 1<<25)
			buf := bytes.NewBuffer(b)
			buf.Reset()
			return buf
		},
	}
)

//go:generate protoc -I=pb --go_out=pb pb/index.proto
func main() {
	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	f, err := os.Create("error.log")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, f))

	pbi, err := Index()
	if err != nil {
		log.Fatal(err)
	}

	BuildTags(pbi)

	// Modules(pbi)

	// idx := index()
	// hosting(idx)
	// versions(idx)

	// latest(idx)
	// whousesweirdcaps()
}

func BuildTags(pbi *pb.Index) {
	mods := make(map[string][]string)
	for _, ir := range pbi.Records {
		mods[ir.Path] = append(mods[ir.Path], ir.Version)
	}
	for _, v := range mods {
		sort.Slice(v, func(i, j int) bool {
			return semver.Compare(v[i], v[j]) == -1
		})
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)
	for i := 0; i < limit; i++ {
		sem <- struct{}{}
	}
	c := make(chan string)
	go func() {
		f, err := os.Create("buildtags.txt")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		for s := range c {
			f.WriteString(s)
			f.WriteString("\n")
		}
	}()

	var i, gone int64
	go func() {
		for range time.NewTicker(10 * time.Second).C {
			log.Printf("progress %d/%d gone %d\n", atomic.LoadInt64(&i), len(mods), atomic.LoadInt64(&gone))
		}
	}()
	for m, v := range mods {
		wg.Add(1)
		<-sem
		atomic.AddInt64(&i, 1)
		go func(m, v string) {
			buf := pool.Get().(*bytes.Buffer)
			defer func() {
				buf.Reset()
				pool.Put(buf)
				wg.Done()
				sem <- struct{}{}
			}()

			res, err := http.Get(fmt.Sprintf("%s/%s/@v/%s.zip", proxyURL, m, v))
			if err != nil {
				log.Printf("module get %s %s: %v", m, v, err)
				return

			} else if res.StatusCode == 410 {
				atomic.AddInt64(&gone, 1)
				return
			} else if res.StatusCode != 200 {
				log.Printf("module status %s %s: %d %s", m, v, res.StatusCode, res.Status)
				return
			}
			defer res.Body.Close()
			_, err = buf.ReadFrom(res.Body)
			if err != nil {
				log.Printf("module read %s %s: %v", m, v, err)
				return
			}
			r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				log.Printf("module unzip %s %s: %v", m, v, err)
				return
			}
			for _, zf := range r.File {
				if filepath.Ext(zf.Name) != ".go" {
					continue
				}
				rc, err := zf.Open()
				if err != nil {
					log.Printf("module open %s %s %s: %v", m, v, zf.Name, err)
					return
				}
				sc := bufio.NewScanner(rc)
				for sc.Scan() {
					if !strings.HasPrefix(sc.Text(), "// +build") {
						continue
					}
					c <- sc.Text()
				}
			}
		}(m, v[len(v)-1])
	}

	wg.Wait()
	close(c)

}
