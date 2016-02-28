package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// Remove files in the data directory that are no longer relevant given the
// configuration file we're using.
func clean(showCount int, cleanc <-chan *cleaningWhitelist) (int, error) {
	keepers := make(map[string]struct{})

	for i := 0; i < showCount; i++ {
		wl := <-cleanc
		for _, p := range wl.paths {
			keepers[p] = struct{}{}
		}
		defer close(wl.cleanFinishedC)
	}

	var rmCount int

	cleanSubdir := func(subd string) error {
		dirContents, err := ioutil.ReadDir(subd)
		if err != nil {
			return err
		}
		for _, info := range dirContents {
			path := filepath.Join(subd, info.Name())
			if _, found := keepers[path]; found {
				continue
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			rmCount++
		}
		return nil
	}

	for _, subd := range []string{dataSubdirEpisodes, dataSubdirMetadata} {
		if err := cleanSubdir(subd); err != nil {
			return rmCount, err
		}
	}
	return rmCount, nil
}

type cleaningWhitelist struct {
	paths          []string
	cleanFinishedC chan struct{}
}

// ------------------------------------------------------------

// To the goroutine cleaning out the data directory, send a whitelist
// containing the paths of files on disk that remain relevant to this watcher,
// so that they will not be removed.
func (w *watcher) sendCleaningWhitelist(vids []ytVidInfo) {
	wlist := cleaningWhitelist{cleanFinishedC: make(chan struct{})}
	for _, vi := range vids {
		wlist.paths = append(wlist.paths,
			vi.episodePath(w.cfg.YTDLWriteExt))
	}
	wlist.paths = append(wlist.paths, w.show.artPath())
	wlist.paths = append(wlist.paths, w.show.feedPath())
	w.cleanc <- &wlist
	<-wlist.cleanFinishedC
}
