package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/seankhliao/gomodstats/retrieve"
)

func main() {
	initLog()
	store, err := retrieve.DefaultClient.Index()
	if err != nil {
		log.Error().Err(err).Msg("index")
		return
	}
	var totalvers int
	for _, mods := range store.Mods {
		totalvers += len(mods)
	}
	log.Info().Int("modules", len(store.Mods)).Int("versions", totalvers).Msg("index")

	b, err := json.Marshal(store)
	if err != nil {
		log.Error().Err(err).Msg("index marshal")
		return
	}
	err = ioutil.WriteFile("index.json", b, 0644)
	if err != nil {
		log.Error().Err(err).Msg("index write")
		return
	}

	var errs []error
	store, errs = retrieve.DefaultClient.ProxyMeta(store)
	if len(errs) != 0 {
		log.Error().Int("total errors", len(errs)).Msg("proxy meta")
		buf := bytes.Buffer{}
		for _, err := range errs {
			buf.WriteString(err.Error() + "\n")
		}
		err = ioutil.WriteFile("proxymeta.log", buf.Bytes(), 0644)
		if err != nil {
			log.Error().Err(err).Msg("proxy meta write log")
		}
	}

	totalvers = 0
	for _, mods := range store.Mods {
		totalvers += len(mods)
	}
	log.Info().Int("modules", len(store.Mods)).Int("versions", totalvers).Msg("proxy meta")
	f, err := os.Create("proxymeta.json")
	if err != nil {
		log.Err(err).Msg("proxy meta create")
	}
	defer f.Close()
	e := json.NewEncoder(f)
	for _, ms := range store.Mods {
		for _, m := range ms {
			e.Encode(m)
		}
	}
}
