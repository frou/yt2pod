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
	fsImpl             http.FileSystem
	hitc               chan string
	period             time.Duration
	periodTicker       *time.Ticker
	forbiddenResources map[string]struct{}
}

func newHitLoggingFsys(
	fsImpl http.FileSystem, period time.Duration) *hitLoggingFsys {

	h := hitLoggingFsys{
		fsImpl:             fsImpl,
		hitc:               make(chan string),
		period:             period,
		forbiddenResources: make(map[string]struct{}),
	}
	h.periodTicker = time.NewTicker(h.period)
	go h.runLoop()
	return &h
}

func (h *hitLoggingFsys) Open(name string) (http.File, error) {
	h.hitc <- name
	if _, ok := h.forbiddenResources[name]; ok {
		return nil, errForbiddenResource
	}
	return h.fsImpl.Open(name)
}

func (h *hitLoggingFsys) Forbid(name string) {
	h.forbiddenResources[name] = struct{}{}
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

var (
	errForbiddenResource = errors.New("forbidden resource")
)
