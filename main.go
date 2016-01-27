package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

const (
	configFileName = "config.json"
	dataDir        = "var"
	epSubdir       = "ep"
	metaSubdir     = "meta"

	downloadAudioFormat = "m4a"
	downloadCmdName     = "youtube-dl"
)

var (
	useSyslog = flag.Bool("syslog", false,
		"send log statements to syslog rather than writing them to stderr")
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
		wat, err := newWatcher(ytAPI, cfg, i,
			time.Duration(cfg.CheckIntervalMinutes)*time.Minute)
		if err != nil {
			log.Fatal(err)
		}
		go wat.watch()
	}

	// Run a webserver to serve the files downloaded and generated.
	log.Fatal(http.ListenAndServe(
		fmt.Sprint(":", cfg.ServePort),
		http.FileServer(http.Dir("."))))
}

func setup() (*config, error) {
	flag.Parse()

	// Setup log destination & format.
	var (
		w     io.Writer
		flags int
	)
	if *useSyslog {
		sender := filepath.Base(os.Args[0])
		var err error
		w, err = syslog.New(syslog.LOG_DAEMON|syslog.LOG_NOTICE, sender)
		if err != nil {
			return nil, err
		}
		//flags = log.Lshortfile
	} else {
		w = os.Stderr
		flags = log.Ldate | log.Ltime //| log.Lshortfile
	}
	log.SetOutput(w)
	log.SetFlags(flags)

	// Up front, check that the youtube-dl command is available.
	if err := exec.Command(downloadCmdName, "--version").Run(); err != nil {
		return nil, errors.New(downloadCmdName + " command is not available")
	}

	// Load config from disk.
	cfg, err := loadConfig(configFileName)
	if err != nil {
		return nil, err
	}

	// Create the data directory.
	err = os.Mkdir(dataDir, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	// Change into it (don't want the webserver to expose our config file).
	if err := os.Chdir(dataDir); err != nil {
		return nil, err
	}
	// Create content sub directories.
	for _, name := range []string{metaSubdir, epSubdir} {
		err := os.Mkdir(name, 0755)
		if err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	return cfg, nil
}
