package main

import (
	"errors"
	"log"
	"net/http"
	"path"
	"time"
)

// Wrap a http.FileSystem to log how many hits it gets every given period.
type hitLoggingFsys struct {
	fsImpl                 http.FileSystem
	hitc                   chan string
	period                 time.Duration
	periodTicker           *time.Ticker
	serveDirectoryListings bool
}

func newHitLoggingFsys(
	fsImpl http.FileSystem,
	period time.Duration,
	serveDirectoryListings bool) *hitLoggingFsys {

	h := hitLoggingFsys{
		fsImpl:                 fsImpl,
		hitc:                   make(chan string),
		period:                 period,
		serveDirectoryListings: serveDirectoryListings,
	}
	h.periodTicker = time.NewTicker(h.period)
	go h.runLoop()
	return &h
}

func (h *hitLoggingFsys) Open(name string) (http.File, error) {
	h.hitc <- name
	f, err := h.fsImpl.Open(name)
	if err != nil {
		return nil, err
	}

	if !h.serveDirectoryListings {
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if stat.IsDir() {
			return nil, errors.New("directory listing has been disallowed")
		}
	}
	return f, nil
}

func (h *hitLoggingFsys) runLoop() {
	for {
		hits := make(map[string]uint) // resource "directory" -> hit count
	ThisPeriod:
		for {
			select {
			case resource := <-h.hitc:
				hits[path.Dir(resource)]++
			case <-h.periodTicker.C:
				log.Printf("Hits in last %v period by dir: %v", h.period, hits)
				break ThisPeriod
			}
		}
	}
}
