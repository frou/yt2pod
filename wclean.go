package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/frou/poor-mans-generics/set"
)

// Remove files in the data directory that are no longer relevant given the
// configuration file we're using.
func clean(podcastCount int, cleanc <-chan *cleaningWhitelist) (int, error) {
	var keepers set.Strings

	for i := 0; i < podcastCount; i++ {
		wl := <-cleanc
		for _, p := range wl.paths {
			keepers.Add(p)
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
			if keepers.Contains(path) {
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
			vi.episodePath(w.fileExtension()))
	}
	wlist.paths = append(wlist.paths, w.pod.artPath())
	wlist.paths = append(wlist.paths, w.pod.feedPath())
	w.cleanc <- &wlist
	<-wlist.cleanFinishedC
}
