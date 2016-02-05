package main

import (
	"log"
	"net/http"
	"path"
	"time"
)

// Wrap a http.FileSystem to log how many hits it gets every given period.
type hitLoggingFsys struct {
	fsImpl       http.FileSystem
	hitc         chan string
	period       time.Duration
	periodTicker *time.Ticker
}

func newHitLoggingFsys(
	fsImpl http.FileSystem, period time.Duration) *hitLoggingFsys {

	h := hitLoggingFsys{
		fsImpl: fsImpl,
		hitc:   make(chan string),
		period: period,
	}
	h.periodTicker = time.NewTicker(h.period)
	go h.runLoop()
	return &h
}

func (h *hitLoggingFsys) Open(name string) (http.File, error) {
	h.hitc <- name
	return h.fsImpl.Open(name)
}

func (h *hitLoggingFsys) runLoop() {
	hits := make(map[string]uint) // resource "directory" -> hit count
	for {
		select {
		case resource := <-h.hitc:
			hits[path.Dir(resource)]++
		case <-h.periodTicker.C:
			log.Printf("Hits in last %v period by dir: %v", h.period, hits)
			hits = make(map[string]uint)
		}
	}
}
