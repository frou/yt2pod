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
	"runtime"
	"time"

	"github.com/frou/stdext"

	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

const (
	downloadAudioFormat = "m4a"
	downloadCmdName     = "youtube-dl"

	dataSubdirEpisodes = "ep"
	dataSubdirMetadata = "meta"

	version = "0.1"
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
		wat, err := newWatcher(ytAPI, cfg, &cfg.Shows[i],
			time.Duration(cfg.CheckIntervalMinutes)*time.Minute)
		if err != nil {
			log.Fatal(err)
		}
		go wat.begin()
	}

	// Run a webserver to serve the episode and metadata files.
	log.Fatal(http.ListenAndServe(
		fmt.Sprint(":", cfg.ServePort),
		http.FileServer(http.Dir("."))))
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

	// Load config from disk.
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return nil, errors.New("config: " + err.Error())
	}
	log.Print("Config loaded from ", *configPath)

	// Up front, check that the youtube-dl command is available.
	output, err := exec.Command(downloadCmdName, "--version").Output()
	if err != nil {
		return nil, errors.New(downloadCmdName + " command is not available")
	}
	log.Printf("Version of %s is %s", downloadCmdName, output)

	// Create the data directory.
	err = os.Mkdir(*dataPath, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	// Change into it (don't want the webserver to expose our config file).
	if err := os.Chdir(*dataPath); err != nil {
		return nil, err
	}
	// Create data sub directories.
	for _, name := range []string{dataSubdirMetadata, dataSubdirEpisodes} {
		err := os.Mkdir(name, 0755)
		if err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	return cfg, nil
}
