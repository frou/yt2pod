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
	"syscall"
	"time"

	"github.com/frou/stdext"
)

func setup() (*config, error) {
	stdext.SetPreFlagsUsageMessage(version, false)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stderr, version)
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
	log.Print(version)

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

	stdext.HandleSignal(syscall.SIGUSR1, true, func() {
		lastTimeAnyFeedWritten.Set(time.Now())
		log.Print("Reset the clock for stale feeds, due to signal")
	})

	return cfg, nil
}
