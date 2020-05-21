package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"go.seankhliao.com/gomodstats/v2/pb"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/proto"
)

const (
	indexURL = "https://index.golang.org/index"
	proxyURL = "https://proxy.golang.org"

	chkptIndex = "index.pb"

	limit = 10
)

var (
	pool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 1<<29)
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

	// f, err := os.Create("error.log")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer f.Close()
	// log.SetOutput(io.MultiWriter(os.Stdout, f))

	pbi, err := Index()
	if err != nil {
		log.Fatal(err)
	}
	// Modules(pbi)

	// idx := index()
	// hosting(idx)
	// versions(idx)

	// latest(idx)
	// whousesweirdcaps()
	timeofday(pbi)
}

func timeofday(pbi *pb.Index) {
	s := make([]int, 60*24)
	for _, r := range pbi.Records {
		t, err := time.Parse(time.RFC3339Nano, r.Timestamp)
		if err != nil {
			log.Println(err)
			continue
		}
		s[t.Hour()*60+t.Minute()]++
	}
	f, err := os.Create("timeofday.csv")
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(f)
	for d, c := range s {
		w.Write([]string{fmt.Sprintf("%02d:%02d", d/60, d%60), strconv.Itoa(c)})
	}
	w.Flush()
	f.Close()
}

func whousesweirdcaps() {
	fis, err := ioutil.ReadDir("mods")
	if err != nil {
		log.Fatal(err)
	}
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}

		b, err := ioutil.ReadFile("mods/" + fi.Name())
		if err != nil {
			log.Println(err)
			continue
		}
		var mv pb.ModuleVersion
		err = proto.Unmarshal(b, &mv)
		if err != nil {
			log.Fatal(err)
		}
		if _, ok := mv.Idents["iNdEx"]; ok {
			fmt.Println("iNdEx: ", fi.Name())
		}
		if _, ok := mv.Idents["dAtA"]; ok {
			fmt.Println("dAtA: ", fi.Name())
		}
	}
}

func latest(idx map[string][]pb.IndexRecord) {
	govers := make(map[string]int64)
	requires := make(map[string]int64)
	replaces := make(map[string]int64)
	excludes := make(map[string]int64)
	tokens := make(map[string]int64)
	tokendist := make(map[string]int64)
	idents := make(map[string]int64)
	identdist := make(map[string]int64)

	for m := range idx {
		ir := idx[m][len(idx[m])-1]

		fn := fmt.Sprintf("%s/%s@%s.pb", "mods", strings.ReplaceAll(ir.Path, "/", "--"), ir.Version)
		b, err := ioutil.ReadFile(fn)
		if err != nil {
			log.Println(err)
			continue
		}

		var mv pb.ModuleVersion
		err = proto.Unmarshal(b, &mv)
		if err != nil {
			log.Fatal(err)
		}
		govers[mv.Go]++
		requires[strconv.Itoa(len(mv.Requires))]++
		replaces[strconv.Itoa(len(mv.Replaces))]++
		excludes[strconv.Itoa(len(mv.Excludes))]++
		var t int64
		for tk, c := range mv.Tokens {
			t += c
			tokens[tk] += c
		}
		tokendist[strconv.FormatInt(t, 10)]++
		var i int64
		for id, c := range mv.Idents {
			i += c
			idents[id] += c
		}
		identdist[strconv.FormatInt(i, 10)]++
	}

	mapcsv("latest-govers.csv", govers)
	mapcsv("latest-requires.csv", requires)
	mapcsv("latest-replaces.csv", replaces)
	mapcsv("latest-excludes.csv", excludes)
	mapcsv("latest-tokenpop.csv", tokens)
	mapcsv("latest-tokencount.csv", tokendist)
	mapcsv("latest-identpop.csv", idents)
	mapcsv("latest-identcount.csv", identdist)

}

func versions(idx map[string][]pb.IndexRecord) {
	modvers := make(map[string]int64)
	prerel := make(map[string]int64)
	for _, irs := range idx {
		modvers[strconv.Itoa(len(irs))]++
		for _, ir := range irs {
			prs := strings.Split(ir.Version, "-")
			for i := 1; i < len(prs); i++ {
				prerel[prs[i]]++
			}
		}
	}

	mapcsv("versions-dist.csv", modvers)
	mapcsv("versions-prerelwords", prerel)
}

func hosting(idx map[string][]pb.IndexRecord) {
	host := make(map[string]int64)
	scm := make(map[string]int64)
	vanity := make(map[string]int64)

	hc := map[string]bool{
		"github.com":         true,
		"bitbucket.org":      true,
		"hub.jazz.net":       true,
		"git.apache.org":     true,
		"git.openstack.org":  true,
		"chiselapp.com":      true,
		"code.launchpad.net": true,
	}

	for k := range idx {
		h := strings.SplitN(k, "/", 2)
		host[h[0]]++
		if !hc[h[0]] {
			if strings.Contains(k, ".git") {
				scm["git"]++
			} else if strings.Contains(k, ".hg") {
				scm["hg"]++
			} else if strings.Contains(k, ".svn") {
				scm["svn"]++
			} else if strings.Contains(k, ".bzr") {
				scm["bzr"]++
			} else if strings.Contains(k, ".fossil") {
				scm["fossil"]++
			} else {
				vanity[h[0]]++
			}
		}
	}
	mapcsv("hosting-all.csv", host)
	mapcsv("hosting-scm.csv", scm)
	mapcsv("hosting-vanity", vanity)
}

func mapcsv(fn string, m map[string]int64) {
	f, err := os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	a := make([][]string, 0, len(m))
	for k, v := range m {
		a = append(a, []string{k, strconv.FormatInt(int64(v), 10)})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i][0] < a[j][0]
	})

	w := csv.NewWriter(f)
	defer w.Flush()
	w.WriteAll(a)
}

func index() map[string][]pb.IndexRecord {
	var pbi pb.Index
	b, err := ioutil.ReadFile(chkptIndex)
	if err != nil {
		log.Fatal(err)
	}
	err = proto.Unmarshal(b, &pbi)
	if err != nil {
		log.Fatal(err)
	}

	m := make(map[string][]pb.IndexRecord, 180000)
	for _, ir := range pbi.Records {
		m[ir.Path] = append(m[ir.Path], *ir)
	}
	for k := range m {
		sort.Slice(m[k], func(i, j int) bool {
			return semver.Compare(m[k][i].Version, m[k][j].Version) == -1
		})
	}

	return m
}
