package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
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

//nolint:gochecknoglobals
var (
	healthConcerns = map[string]healthFunc{
		"disk_low":    diskLow,
		"ytdl_old":    downloaderOld,
		"feeds_stale": feedsStale,
	}

	lastDownloaderVersionCheck struct {
		mu     sync.Mutex
		when   time.Time
		result string
	}
)

const (
	// @todo #0 Make diskLowThreshold & ytdlOldThreshold customizable in the config file.
	diskLowThreshold       = 1024 * 1024 * 1024  // 1GB
	downloaderOldThreshold = time.Hour * 24 * 60 // 60 days
	// @todo #0 Make feedsStaleThreshold customizable per-podcast in the config file.
	feedsStaleThreshold = time.Hour * 24 * 10 // 10 days

	downloaderVersionCheckCacheDuration = time.Minute * 5
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

var yearMonthDayRevnumVersionRE = regexp.MustCompile(`^(\d+\.\d+\.\d+)(\.\d+)?$`)

func downloaderOld() (bool, error) {
	var version string
	lastDownloaderVersionCheck.mu.Lock()
	defer lastDownloaderVersionCheck.mu.Unlock()
	if time.Since(lastDownloaderVersionCheck.when) < downloaderVersionCheckCacheDuration {
		version = lastDownloaderVersionCheck.result
	} else {
		var err error
		version, err = getDownloaderCommandVersion()
		if err != nil {
			return false, err
		}
		lastDownloaderVersionCheck.when = time.Now()
		lastDownloaderVersionCheck.result = version
	}

	submatches := yearMonthDayRevnumVersionRE.FindStringSubmatch(version)
	if submatches == nil {
		return false, fmt.Errorf("Can't parse downloader command's version output %q because it has an unexpected format", version)
	}

	versionTime, err := time.Parse("2006.1.2", submatches[1])
	if err != nil {
		return false, err
	}
	age := time.Since(versionTime)
	return age > downloaderOldThreshold, nil
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

//nolint:gochecknoglobals
var lastTimeAnyFeedWritten = newConcTime()
