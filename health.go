package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/frou/stdext"
	"github.com/tzdybal/go-disk-usage/du"
)

// Define HTTP handlers that an automated monitoring system can access to keep
// an eye on the health of the daemon.
//
// e.g.
//
// Request:
//     /health/disk_low
// Response:
//     CONCERN
//
// Request:
//     /health
// Response:
//    disk_low      OK
//    ytdl_old      CONCERN
//    feeds_stale   OK

const (
	httpHealthPrefix = "/health/"
)

var healthConcerns = map[string]healthFunc{
	"disk_low":    diskLow,
	"ytdl_old":    ytdlOld,
	"feeds_stale": feedsStale,
}

const (
	// @todo #0 Make diskLowThreshold & ytdlOldThreshold customizable in the config file.
	diskLowThreshold = 1024 * 1024 * 1024  // 1GB
	ytdlOldThreshold = time.Hour * 24 * 60 // 60 days
	// @todo #0 Make feedsStaleThreshold customizable per-podcast in the config file.
	feedsStaleThreshold = time.Hour * 24 * 10 // 10 days
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, httpHealthPrefix)

	check := func(name string, f healthFunc) string {
		flag, err := f()
		if err != nil {
			log.Printf("health: %v: %v", name, err)
		}
		if err != nil || flag {
			return "CONCERN"
		}
		return "OK"
	}

	f, found := healthConcerns[name]
	if found {
		fmt.Fprintln(w, check(name, f))
	} else {
		if name == "" {
			for name, f := range healthConcerns {
				fmt.Fprintln(w, name, "\t", check(name, f))
			}
		} else {
			http.NotFound(w, r)
		}
	}
}

// ------------------------------------------------------------

// The bool flag being true or there being an error means cause for concern.
type healthFunc func() (bool, error)

func diskLow() (bool, error) {
	ok := du.NewDiskUsage(".").Available() < diskLowThreshold
	return ok, nil
}

// @todo Update parsing of version number in health check, because yt-dlp at least can have a revision number after the YMD string
// @body e.g. 2021.11.10.1
// @body REF: https://github.com/yt-dlp/yt-dlp/blob/master/devscripts/update-version.py
// @body REF: https://github.com/yt-dlp/yt-dlp/blob/master/yt_dlp/version.py
func ytdlOld() (bool, error) {
	return false, nil
	// @todo #0 Cache ytdl version output for a while to prevent reqs to /health causing DoS.
	//  Because each request currently forks the ytdl process that takes ~2s to run.
	// version, err := exec.Command(downloadCmdName, "--version").Output()
	// if err != nil {
	// 	return false, err
	// }

	// versionTime, err := time.Parse(
	// 	"2006.1.2",
	// 	strings.TrimSpace(string(version)))
	// if err != nil {
	// 	return false, err
	// }
	// age := time.Since(versionTime)
	// return age > ytdlOldThreshold, nil
}

func feedsStale() (bool, error) {
	return time.Since(lastTimeAnyFeedWritten.Get()) > feedsStaleThreshold, nil
}

// ------------------------------------------------------------

type concTime struct {
	a *stdext.ConcAtom
}

func newConcTime() *concTime {
	var zval time.Time
	return &concTime{a: stdext.NewConcAtom(zval)}
}

func (t *concTime) Get() time.Time {
	return t.a.Deref().(time.Time)
}

func (t *concTime) Set(val time.Time) {
	t.a.Replace(val)
}

var lastTimeAnyFeedWritten = newConcTime()
