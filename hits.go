package main

import (
	"log"
	"net/http"
	"time"
)

// Wrap a http.FileSystem to log how many hits it gets every given period.
type hitLoggingFsys struct {
	fsImpl       http.FileSystem
	hitc         chan struct{}
	period       time.Duration
	periodTicker *time.Ticker
}

func newHitLoggingFsys(
	fsImpl http.FileSystem, period time.Duration) *hitLoggingFsys {

	h := hitLoggingFsys{
		fsImpl: fsImpl,
		hitc:   make(chan struct{}),
		period: period,
	}
	h.periodTicker = time.NewTicker(h.period)
	go h.runLoop()
	return &h
}

func (h *hitLoggingFsys) Open(name string) (http.File, error) {
	h.hitc <- struct{}{}
	return h.fsImpl.Open(name)
}

func (h *hitLoggingFsys) runLoop() {
	var n int
	for {
		select {
		case <-h.periodTicker.C:
			log.Printf("Hits in last %v period: %d", h.period, n)
			n = 0
		case <-h.hitc:
			n++
		}
	}
}

// TODO: Separate the hit counts of /meta/* and /ep/*
