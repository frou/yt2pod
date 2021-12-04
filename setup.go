package main

import (
	"bytes"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/frou/stdext"
	"github.com/frou/yt2pod/internal/xplatform"
)

func setup() (*config, error) {
	flag.Parse()

	if *printVersion {
		fmt.Fprintln(os.Stdout, "Version\t", stampedBuildVersion)
		fmt.Fprintln(os.Stdout, "Built\t", stampedBuildTime)
		os.Exit(0)
	}

	// Setup log destination & format.
	var (
		w     io.Writer
		flags int
		err   error
	)
	if *useSyslog {
		executableName := filepath.Base(os.Args[0])
		w, err = xplatform.NewSyslog(executableName)
		if err != nil {
			return nil, err
		}
		// flags = log.Lshortfile
	} else {
		w = os.Stderr
		flags = log.Ldate | log.Ltime //| log.Lshortfile
	}
	log.SetOutput(w)
	log.SetFlags(flags)
	log.Printf("Version %s", stampedBuildVersion)

	// Load config from disk.
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return nil, errors.New("config: " + err.Error())
	}
	log.Print("Config successfully loaded from ", *configPath)

	// Store a closure over cfg, so that the `downloader_old` health check can also make use of this function.
	getDownloaderCommandVersion = func() (string, error) {
		versionBytes, err := exec.Command(cfg.DownloaderName, "--version").Output()
		if err != nil {
			return "", err
		}
		return string(bytes.TrimSpace(versionBytes)), nil
	}
	// Log the name and version of the downloader command that's configured/available.
	version, err := getDownloaderCommandVersion()
	if err != nil {
		// This also catches a custom downloader_name being set in the config file, but that command not existing on PATH.
		return nil, fmt.Errorf("Couldn't determine configured downloader command's version: %w", err)
	}
	log.Printf("Downloader command is %s (currently version %s)", cfg.DownloaderName, version)

	// Up front, check that a GET to a http_s_ server works (which needs CA
	// certs to be present and correct in the OS)
	secureResp, err := http.Get("https://www.googleapis.com/")
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			if sysRootsErr, ok := urlErr.Err.(x509.SystemRootsError); ok {
				return nil, sysRootsErr
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

	xplatform.RegisterStalenessResetter(func() {
		lastTimeAnyFeedWritten.Set(time.Now())
		log.Print("The clock for stale feeds was reset")
	})

	return cfg, nil
}

//nolint:gochecknoglobals
var getDownloaderCommandVersion func() (string, error)
