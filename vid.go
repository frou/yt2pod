package main

import (
	"fmt"
	"path/filepath"
	"time"
)

type ytVidInfo struct {
	id        string
	published time.Time
	title     string
	desc      string
}

func (vi *ytVidInfo) episodePath(fileExt string) string {
	return filepath.Join(dataSubdirEpisodes, fmt.Sprint(vi.id, ".", fileExt))
}

// ------------------------------------------------------------

type vidsChronoSorter []ytVidInfo

func (v vidsChronoSorter) Len() int {
	return len(v)
}

func (v vidsChronoSorter) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

func (v vidsChronoSorter) Less(i, j int) bool {
	return v[i].published.Before(v[j].published)
}
