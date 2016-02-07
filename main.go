package main

import (
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/frou/stdext"

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

	performClean = flag.Bool("clean", false,
		"Remove files in the data directory now irrelevant based on config")

	showVersion = flag.Bool("version", false,
		"show version information then exit")

	versionLabel = fmt.Sprintf("yt2pod v%s", version)
)

const (
	hitLoggingPeriod = 24 * time.Hour
)

func main() {
	cfg, err := setup()
	if err != nil {
		log.Fatal(err)
	}

	apiKey := cfg.YTDataAPIKey
	log.Printf("Using YouTube Data API key ending %s", apiKey[len(apiKey)-5:])

	var cleanc chan *cleaningWhitelist
	if *performClean {
		cleanc = make(chan *cleaningWhitelist)
	}

	for i := range cfg.Shows {
		ytAPI, err := youtube.New(&http.Client{
			Transport: &transport.APIKey{Key: apiKey},
		})
		if err != nil {
			log.Fatal(err)
		}
		wat, err := newWatcher(ytAPI, cfg, &cfg.Shows[i], cleanc)
		if err != nil {
			log.Fatal(err)
		}
		go wat.watch()
	}

	if *performClean {
		n, err := clean(len(cfg.Shows), cleanc)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Clean removed %d files", n)
	}

	// Run a webserver to serve the episode and metadata files.
	hfs := newHitLoggingFsys(http.Dir("."), hitLoggingPeriod)
	log.Fatal(http.ListenAndServe(
		fmt.Sprint(":", cfg.ServePort),
		http.FileServer(hfs)))
}

func setup() (*config, error) {
	stdext.SetPreFlagsUsageMessage(
		versionLabel + " :: https://github.com/frou/yt2pod")
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stderr, versionLabel)
		os.Exit(0)
	}

	// Setup log destination & format.
	var (
		w     io.Writer
		flags int
		err   error
	)
	if *useSyslog {
		sender := filepath.Base(os.Args[0])
		severity := syslog.LOG_INFO
		if runtime.GOOS == "darwin" {
			// OS X processes swallow LOG_INFO and LOG_DEBUG msgs by default.
			severity = syslog.LOG_NOTICE
		}
		w, err = syslog.New(syslog.LOG_DAEMON|severity, sender)
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
	log.Print(versionLabel)

	// Load config from disk.
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return nil, errors.New("config: " + err.Error())
	}
	log.Print("Config successfully loaded from ", *configPath)

	// Up front, check that the youtube downloading command is available.
	output, err := exec.Command(downloadCmdName, "--version").Output()
	if err != nil {
		return nil, errors.New(downloadCmdName + " command is not available")
	}
	log.Printf("Version of %s is %s", downloadCmdName, output)

	// Up front, check that a GET to a http_s_ server works (which needs CA
	// certs to be present and correct in the OS)
	secureResp, err := http.Get("https://www.googleapis.com/")
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			if sysRootsErr, ok := urlErr.Err.(x509.SystemRootsError); ok {
				log.Fatal(sysRootsErr)
			}
		}
		log.Print(err)
	} else {
		secureResp.Body.Close()
	}

	// Create the data directory.
	err = os.Mkdir(*dataPath, stdext.OwnerWritableDir)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	// Change into it (don't want to expose our config file when webserving).
	if err := os.Chdir(*dataPath); err != nil {
		return nil, err
	}
	// Create its subdirectories.
	for _, name := range []string{dataSubdirMetadata, dataSubdirEpisodes} {
		err := os.Mkdir(name, stdext.OwnerWritableDir)
		if err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	return cfg, nil
}
