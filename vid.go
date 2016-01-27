package main

import (
	"path/filepath"
	"time"
)

type ytVidInfo struct {
	id        string
	published time.Time
	title     string
	desc      string
}

func (vi *ytVidInfo) episodePath() string {
	return filepath.Join(epSubdir, vi.id)
}

func (vi *ytVidInfo) String() string {
	return vi.id
}
