package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

func main() {
	tags := map[string]bool{
		"!aix":       true,
		"!android":   true,
		"!darwin":    true,
		"!dragonfly": true,
		"!freebsd":   true,
		"!hurd":      true,
		"!illumos":   true,
		"!linux":     true,
		"!netbsd":    true,
		"!openbsd":   true,
		"!solaris":   true,
		"!js":        true,
		"!nacl":      true,
		"!plan9":     true,
		"!windows":   true,
		"!zos":       true,
		"aix":        true,
		"android":    true,
		"darwin":     true,
		"dragonfly":  true,
		"freebsd":    true,
		"hurd":       true,
		"illumos":    true,
		"linux":      true,
		"netbsd":     true,
		"openbsd":    true,
		"solaris":    true,
		"js":         true,
		"nacl":       true,
		"plan9":      true,
		"windows":    true,
		"zos":        true,
	}

	f, err := os.Open("./buildtags.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var c int
	m := make(map[string]int)

	s := bufio.NewScanner(f)
scan:
	for s.Scan() {
		bts := strings.Fields(strings.TrimPrefix(s.Text(), "// +build "))
		for _, t := range bts {
			if _, ok := tags[t]; !ok {
				continue scan
			}
		}

		c++

		sort.Strings(bts)
		m[strings.Join(bts, " ")]++
	}

	type out struct {
		c int
		t string
	}
	outs := make([]out, 0, len(m))
	for t, c := range m {
		outs = append(outs, out{c, t})
	}
	sort.Slice(outs, func(i, j int) bool {
		return outs[i].c > outs[j].c
	})
	for i := range outs {
		fmt.Printf("% 8d %s\n", outs[i].c, outs[i].t)
	}
	fmt.Printf("\ntotal %d lines\n", c)
}
