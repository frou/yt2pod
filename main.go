package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

const (
	dataSubdirEpisodes = "ep"
	dataSubdirMetadata = "meta"

	downloadCmdName = "youtube-dl"

	version = "0.9.5"
)

var (
	useSyslog = flag.Bool("syslog", false,
		"send log statements to syslog rather than writing them to stderr")

	configPath = flag.String("config", "config.json",
		"path to config file")

	dataPath = flag.String("data", "data",
		"path to directory to change into and write data (created if needed)")

	showVersion = flag.Bool("version", false,
		"show version information then exit")

	versionLabel = fmt.Sprintf("yt2pod v%s", version)

	hitLoggingPeriod = 24 * time.Hour
)

func main() {
	cfg, err := setup()
	if err != nil {
		log.Fatal(err)
	}

	apiKey := cfg.YTDataAPIKey
	log.Printf("Using YouTube Data API key ending %s", apiKey[len(apiKey)-5:])
	for i := range cfg.Shows {
		ytAPI, err := youtube.New(&http.Client{
			Transport: &transport.APIKey{Key: apiKey},
		})
		if err != nil {
			log.Fatal(err)
		}
		wat, err := newWatcher(ytAPI, cfg, &cfg.Shows[i])
		if err != nil {
			log.Fatal(err)
		}
		go wat.begin()
	}

	// Run a webserver to serve the episode and metadata files.
	files := newHitLoggingFsys(http.Dir("."), hitLoggingPeriod)
	websrv := http.Server{
		Addr:    fmt.Sprint(":", cfg.ServePort),
		Handler: http.FileServer(files),
		// Conserve # open FDs by pruning persistent (keep-alive) HTTP conns.
		ReadTimeout: 15 * time.Second,
	}
	log.Fatal(websrv.ListenAndServe())
}
