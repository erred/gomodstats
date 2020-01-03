package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

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
			buf.WriteString(err.Error())
		}
		err = ioutil.WriteFile("proxymeta.log", buf.Bytes(), 0644)
		if err != nil {
			log.Error().Err(err).Msg("proxy meta write log")
		}
	}
	b, err = json.Marshal(store)
	if err != nil {
		log.Error().Err(err).Msg("proxy meta marshal")
		return
	}
	err = ioutil.WriteFile("proxymeta.json", b, 0644)
	if err != nil {
		log.Error().Err(err).Msg("proxy meta write")
	}

}
