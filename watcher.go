package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/jbub/podcasts"
)

type watcher struct {
	ytAPI         *youtube.Service
	cfg           *config
	show          *show
	checkInterval time.Duration

	lastChecked  time.Time
	ytAPIRespite time.Duration
	vids         []ytVidInfo
}

func newWatcher(ytAPI *youtube.Service, cfg *config, show *show,
	checkInterval time.Duration) (*watcher, error) {

	w := watcher{
		ytAPI:         ytAPI,
		cfg:           cfg,
		show:          show,
		checkInterval: checkInterval,
	}
	// Up front, check that the YouTube API is working. Do this by fetching the
	// name of the channel and its 'avatar' image (both made use of later).
	err := w.getChannelInfo()
	return &w, err
}

func (w *watcher) begin() {
	initialCheck, problemVids := true, make(map[string]ytVidInfo)
	for {
		// Sleep until it's time for a check.
		elapsed := time.Since(w.lastChecked)
		if elapsed < w.checkInterval {
			time.Sleep(w.checkInterval - elapsed)
		}

		// After the initial check, only vids published since the last check
		// need be queried for.
		var pubdAfter time.Time
		if initialCheck {
			pubdAfter = w.show.Epoch
			if !w.show.Epoch.IsZero() {
				log.Printf("%s: Epoch is configured as %s",
					w.show, w.show.EpochStr)
			}
		} else {
			pubdAfter = w.lastChecked
		}

		latestVids, err := w.getLatestVids(pubdAfter)
		if err != nil {
			log.Printf("%s: Getting latest vids failed: %v", w.show, err)
			if w.ytAPIRespite > 0 {
				log.Printf("%s: Giving YouTube API %v respite",
					w.show, w.ytAPIRespite)
				time.Sleep(w.ytAPIRespite)
				w.ytAPIRespite = 0
			}
			continue
		}
		if len(latestVids) == 0 &&
			// During the initial check, even if there are no vids, the feed
			// file still needs to be written (in case it doesn't exist yet).
			!initialCheck {
			continue
		}
		w.vids = append(w.vids, latestVids...)
		log.Printf("%s: %d vids of interest were published (now %d in total)",
			w.show, len(latestVids), len(w.vids))
		for _, vi := range latestVids {
			if err := w.download(vi); err != nil {
				log.Printf("%s: Download failed: %v", w.show, err)
				problemVids[vi.id] = vi
			}
		}

		// Try and resolve vids that had download problems during this (and
		// previous) checks.
		if n := len(problemVids); n > 0 {
			log.Printf("%s: There are %d problem vids", w.show, n)
		}
		for _, vi := range problemVids {
			err := w.download(vi)
			if err == nil {
				delete(problemVids, vi.id)
				log.Printf("%s: Resolved problem vid %s", w.show, vi.id)
			}
		}

		// Write the podcast feed XML to disk.
		if err := w.writeFeed(); err != nil {
			log.Printf("%s: Writing feed failed: %v", w.show, err)
		}
		if initialCheck {
			log.Printf("%s: URL for feed is configured as %s",
				w.show, w.cfg.urlFor(w.show.feedPath()))
		}
		initialCheck = false
	}
}

func (w *watcher) download(vi ytVidInfo) error {
	diskPath := vi.episodePath()
	if _, err := os.Stat(diskPath); err == nil {
		// log.Printf("%s: %s already downloaded", w.show, vi.id)
		return nil
	}

	line := fmt.Sprintf("%s -f %s -o %s -- %s",
		downloadCmdName, downloadAudioFormat, diskPath, vi.id)
	log.Printf("%s: Running: %s", w.show, line)

	var stderr bytes.Buffer
	lineSplit := strings.Split(line, " ")
	cmd := exec.Command(lineSplit[0], lineSplit[1:]...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("%v: %s", err, stderr.String())
	}
	return err
}

func (w *watcher) writeFeed() error {
	// Construct the feed description blurb. It's kept shorter when the show
	// has less configuration.
	feedDesc := new(bytes.Buffer)
	fmt.Fprintf(feedDesc,
		"Generated based on the videos of YouTube channel \"%s\"",
		w.show.YTReadableChannelName)
	if !w.show.Epoch.IsZero() {
		fmt.Fprintf(feedDesc, " published from %s onwards", w.show.EpochStr)
	}
	if w.show.TitleFilterStr != "" {
		fmt.Fprintf(feedDesc, " with titles matching \"%s\"",
			w.show.TitleFilterStr)
	}
	fmt.Fprintf(feedDesc, " [%s]", versionLabel)

	feedBuilder := &podcasts.Podcast{
		Title:       w.show.Name,
		Link:        "https://www.youtube.com/channel/" + w.show.YTChannelID,
		Copyright:   w.show.YTReadableChannelName,
		Language:    "en",
		Description: feedDesc.String(),
	}

	audioType := "audio/" + downloadAudioFormat
	for _, vi := range w.vids {
		var epSize int64
		f, err := os.Open(vi.episodePath())
		if err == nil {
			info, err := f.Stat()
			if err == nil {
				epSize = info.Size()
			}
			f.Close()
		}

		feedBuilder.AddItem(&podcasts.Item{
			Title:   vi.title,
			Summary: vi.desc,
			GUID:    w.cfg.urlFor(vi.episodePath()),
			PubDate: &podcasts.PubDate{Time: vi.published},
			Enclosure: &podcasts.Enclosure{
				URL:    w.cfg.urlFor(vi.episodePath()),
				Length: fmt.Sprint(epSize),
				Type:   audioType,
			},
		})
	}

	applyImg := podcasts.Image(w.cfg.urlFor(w.show.artPath()))
	feed, err := feedBuilder.Feed(applyImg)
	if err != nil {
		return err
	}
	f, err := os.Create(w.show.feedPath())
	if err != nil {
		return err
	}
	defer f.Close()
	log.Printf("%s: Writing out feed", w.show)
	if err := feed.Write(f); err != nil {
		return err
	}
	fmt.Fprintln(f)
	return nil
}

func (w *watcher) getLatestVids(pubdAfter time.Time) ([]ytVidInfo, error) {
	var (
		latestVids    []ytVidInfo
		nextPageToken string
	)
	for {
		apiReq := w.ytAPI.Search.List("id,snippet").
			ChannelId(w.show.YTChannelID).
			Type("video").
			PublishedAfter(pubdAfter.Format(time.RFC3339)).
			Order("date").
			MaxResults(50).
			PageToken(nextPageToken)
		checkTime := time.Now()
		apiResp, err := apiReq.Do()
		if err != nil {
			w.ytAPIRespite = 5 * time.Minute
			return nil, err
		}
		w.lastChecked = checkTime
		for _, item := range apiResp.Items {
			if item.Id.Kind != "youtube#video" {
				return nil, errors.New("non-video in response items")
			}
			if !w.show.TitleFilter.MatchString(item.Snippet.Title) {
				continue
			}
			pubd, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			if err != nil {
				return nil, err
			}
			vi := ytVidInfo{
				id:        item.Id.VideoId,
				published: pubd,
				title:     item.Snippet.Title,
				desc:      item.Snippet.Description,
			}
			latestVids = append(latestVids, vi)
		}
		nextPageToken = apiResp.NextPageToken
		if nextPageToken == "" {
			break
		}
	}
	return latestVids, nil
}

func (w *watcher) getChannelInfo() error {
	apiReq := w.ytAPI.Channels.List("snippet").
		Id(w.show.YTChannelID).
		MaxResults(1)
	apiResp, err := apiReq.Do()
	if err != nil {
		return err
	}
	if len(apiResp.Items) != 1 || apiResp.Items[0].Kind != "youtube#channel" {
		return errors.New("expected exactly 1 channel in response items")
	}

	item := apiResp.Items[0]
	w.show.YTReadableChannelName = item.Snippet.Title

	thumbURL := item.Snippet.Thumbnails.High.Url
	thumbResp, err := http.Get(thumbURL)
	if err != nil {
		return err
	}
	defer thumbResp.Body.Close()
	buf, err := ioutil.ReadAll(thumbResp.Body)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(w.show.artPath(), buf, 0644)
}
