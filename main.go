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

	version = "1.3.1-pre"

	hitLoggingPeriod       = 24 * time.Hour
	websrvClientReadTimout = 15 * time.Second
	ytAPIRespiteUnit       = 5 * time.Minute
)

var (
	useSyslog = flag.Bool("syslog", false,
		"send log statements to syslog rather than writing them to stderr")

	configPath = flag.String("config", "config.json",
		"path to config file")

	dataPath = flag.String("data", "data",
		"path to directory to change into and write data (created if needed)")

	dataClean = flag.Bool("dataclean", false,
		"during initialisation, remove files in the data directory that are irrelevant given the current config")

	showVersion = flag.Bool("version", false,
		"show version information then exit")

	versionLabel = fmt.Sprintf("yt2pod v%s", version)
)

func main() {
	cfg, err := setup()
	if err != nil {
		log.Fatal(err)
	}

	err = run(cfg)
	if err != nil {
		log.Fatal(err)
	}
}

func run(cfg *config) error {
	apiKey := cfg.YTDataAPIKey
	log.Printf("Using YouTube Data API key ending %s", apiKey[len(apiKey)-5:])

	var cleanc chan *cleaningWhitelist
	if *dataClean {
		cleanc = make(chan *cleaningWhitelist)
	}

	for i := range cfg.Podcasts {
		ytAPI, err := youtube.New(&http.Client{
			Transport: &transport.APIKey{Key: apiKey},
		})
		if err != nil {
			return err
		}
		wat, err := newWatcher(
			ytAPI, cfg.watcherConfig, &cfg.Podcasts[i], cleanc)
		if err != nil {
			log.Fatal(err)
		}
		go wat.watch()
	}

	if *dataClean {
		n, err := clean(len(cfg.Podcasts), cleanc)
		if err != nil {
			return err
		}
		log.Printf("Clean removed %d files", n)
	}

	// Run a webserver to serve the episode and metadata files.

	mux := http.NewServeMux()

	// TODO^: Look for this in cfg. Missing/default is false
	forbidDirSnoop := true

	files := newHitLoggingFsys(http.Dir("."), hitLoggingPeriod)
	if forbidDirSnoop {
		dirNames := []string{"", dataSubdirEpisodes, dataSubdirMetadata}
		for _, name := range dirNames {
			files.Forbid("/" + name)
		}
	}
	mux.Handle("/", http.FileServer(files))

	mux.HandleFunc(httpHealthPrefix, healthHandler)

	websrv := http.Server{
		Addr:    fmt.Sprint(cfg.ServeHost, ":", cfg.ServePort),
		Handler: mux,
		// Conserve # open FDs by pruning persistent (keep-alive) HTTP conns.
		ReadTimeout: websrvClientReadTimout,
	}
	err := websrv.ListenAndServe()
	if err != nil {
		samePortAllInterfaces := fmt.Sprint(":", cfg.ServePort)
		log.Printf("Web server could not listen on %v, trying %v instead",
			websrv.Addr, samePortAllInterfaces)
		websrv.Addr = samePortAllInterfaces
		err = websrv.ListenAndServe()
	}
	return err
}
