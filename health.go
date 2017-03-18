package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/frou/stdext"
)

// Define HTTP handlers that an automated monitoring system can access to keep
// an eye on the health of the daemon.
//
// e.g.
//
// Request:
//     /health/disk_low
// Reponse:
//     CONCERN
//
// Request:
//     /health
// Reponse:
//    disk_low      OK
//    ytdl_old      CONCERN
//    feeds_stale   OK

const (
	httpHealthPrefix = "/health/"
)

var healthConcerns = map[string]healthFunc{
	// TODO: Use reflection to construct the resource names from the func nms?
	"disk_low":    diskLow,
	"ytdl_old":    ytdlOld,
	"feeds_stale": feedsStale,
}

// TODO: Allow these to be overriden by cfg file.
const (
	// TODO: Should this be absolute number of bytes or percentage of disk?
	diskLowThreshold    = 1024 * 1024 * 1024  // 1GB
	ytdlOldThreshold    = time.Hour * 24 * 60 // 60 days
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
		} else {
			return "OK"
		}
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
	var t syscall.Statfs_t
	err := syscall.Statfs(".", &t)
	if err != nil {
		return false, err
	}
	bytesAvailable := uint64(t.Bsize) * t.Bavail
	return bytesAvailable < diskLowThreshold, nil
}

func ytdlOld() (bool, error) {
	// TODO: Cache this for ... minutes because otherwise requesting /health
	// could be a DoS because every request forks a process that takes ~2s to
	// run.
	version, err := exec.Command(downloadCmdName, "--version").Output()
	if err != nil {
		return false, err
	}
	versionTime, err := time.Parse(
		"2006.1.2",
		strings.TrimSpace(string(version)))
	if err != nil {
		return false, err
	}
	age := time.Since(versionTime)
	return age > ytdlOldThreshold, nil
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
