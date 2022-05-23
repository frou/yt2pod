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
	"runtime/debug"
	"time"

	"github.com/frou/stdext"
	"github.com/frou/yt2pod/internal/xplatform"
)

func introspectOwnVersion() string {
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		var vcs, vcsRevision, vcsModified *string
		for _, kvp := range buildInfo.Settings {
			value := kvp.Value
			switch kvp.Key {
			case "vcs":
				vcs = &value
			case "vcs.revision":
				vcsRevision = &value
			case "vcs.modified":
				vcsModified = &value
			}
		}
		if vcs != nil && vcsRevision != nil && vcsModified != nil {
			dirtyIndicator := ""
			if *vcsModified == "true" {
				dirtyIndicator = "-dirty"
			}
			// Abbreviate the SHA to seven hex characters like git itself does.
			return fmt.Sprintf("%s-%s%s", *vcs, (*vcsRevision)[:7], dirtyIndicator)
		}
	}
	return "unknown-version"
}

func setup() (*config, error) {
	flag.Parse()

	ownVersion := introspectOwnVersion()
	if *flagPrintVersion {
		fmt.Println("Version:", ownVersion)
		os.Exit(0)
	}

	// Setup log destination & format.
	var (
		w     io.Writer
		flags int
		err   error
	)
	if *flagUseSyslog {
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
	log.Printf("Version: %s", ownVersion)

	// Load config from disk.
	cfg, err := loadConfig(*flagConfigPath)
	if err != nil {
		return nil, errors.New("config: " + err.Error())
	}
	log.Print("Config successfully loaded from ", *flagConfigPath)

	// Store a closure over cfg, so that the `downloaderOld` health check can also make use of this function.
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
		var urlErr *url.Error
		var sysRootsErr *x509.SystemRootsError
		if errors.As(err, &urlErr) && errors.As(urlErr.Err, &sysRootsErr) {
			return nil, sysRootsErr
		}
		// Not the specific thing we are proactively checking for, so just log it and continue.
		log.Print(err)
	} else {
		secureResp.Body.Close()
	}

	// Create the data directory.
	err = os.Mkdir(*flagDataPath, stdext.OwnerWritableDir)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	// Change into it (don't want to expose our config file when webserving).
	if err := os.Chdir(*flagDataPath); err != nil {
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
