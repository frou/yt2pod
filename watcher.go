package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/frou/podcasts"
	"github.com/frou/stdext"
	"github.com/nfnt/resize"
)

type watcher struct {
	ytAPI         *youtube.Service
	cfg           watcherConfig
	pod           *podcast
	checkInterval time.Duration

	initialCheck bool
	lastChecked  time.Time
	ytAPIRespite time.Duration
	vids         []ytVidInfo

	problemVids map[string]ytVidInfo
	cleanc      chan *cleaningWhitelist
}

func newWatcher(ytAPI *youtube.Service, cfg watcherConfig, pod *podcast,
	cleanc chan *cleaningWhitelist) (*watcher, error) {

	w := watcher{
		ytAPI:         ytAPI,
		cfg:           cfg,
		pod:           pod,
		checkInterval: time.Duration(cfg.CheckIntervalMinutes) * time.Minute,

		initialCheck: true,
		problemVids:  make(map[string]ytVidInfo),
		cleanc:       cleanc,
	}

	// Make a video podcast (720p or 360p) instead of a normal audio podcast.
	// This is undocumented for now.
	if w.pod.Vidya {
		w.cfg.YTDLFmtSelector = "22/18"
		w.cfg.YTDLWriteExt = "mp4"
	}

	// Up front, check that the YouTube API is working. Do this by fetching the
	// name of the channel and its 'avatar' image (both made use of later).
	err := w.getChannelInfo()
	return &w, err
}

func (w *watcher) watch() {
	for {
		// Sleep until it's time for a check.
		elapsed := time.Since(w.lastChecked)
		if elapsed < w.checkInterval {
			time.Sleep(w.checkInterval - elapsed)
		}

		// The initial check does a full query for vids. Subsequent checks need
		// only query vids published after the last check.
		var pubdAfter time.Time
		if w.initialCheck {
			pubdAfter = w.pod.Epoch
			if !w.pod.Epoch.IsZero() {
				log.Printf("%s: Epoch is configured as %s",
					w.pod, w.pod.EpochStr)
			}
			// Write out the feed early. Even though it contains no items yet,
			// it's better that the XML file exist in some form vs 404ing.
			if err := w.writeFeed(); err != nil {
				log.Printf("%s: Writing feed failed: %v", w.pod, err)
			}
		} else {
			pubdAfter = w.lastChecked
		}

		// Do the check.
		latestVids, err := w.getLatest(pubdAfter)
		if err != nil {
			log.Printf("%s: Getting latest vids failed: %v", w.pod, err)
			if w.ytAPIRespite > 0 {
				log.Printf("%s: Giving YouTube API %v respite",
					w.pod, w.ytAPIRespite)
				time.Sleep(w.ytAPIRespite)
				w.ytAPIRespite = 0
			}
			continue
		}

		if *dataClean && w.initialCheck {
			w.sendCleaningWhitelist(latestVids)
		}

		w.processLatest(latestVids)
		w.initialCheck = false
	}
}

func (w *watcher) processLatest(latestVids []ytVidInfo) {
	w.vids = append(w.vids, latestVids...)

	areNewVids := len(latestVids) > 0
	if areNewVids {
		log.Printf("%s: %d vids of interest published (makes %d in total)",
			w.pod, len(latestVids), len(w.vids))
	}
	var areNewProblems, problemResolved bool
	for _, vi := range latestVids {
		if err := w.download(vi, true); err != nil {
			log.Printf("%s: %s download failed: %v", w.pod, vi.id, err)
			w.problemVids[vi.id] = vi
			areNewProblems = true
		}
	}

	// Try and resolve vids that had download problems during this (and
	// previous) checks.
	if areNewProblems {
		log.Printf("%s: There are now %d problem vids",
			w.pod, len(w.problemVids))
	}
	for _, vi := range w.problemVids {
		err := w.download(vi, false)
		if err == nil {
			delete(w.problemVids, vi.id)
			problemResolved = true
			log.Printf("%s: Resolved problem vid %s", w.pod, vi.id)
		}
	}

	// Write the podcast feed XML to disk.
	if areNewVids || problemResolved {
		if err := w.writeFeed(); err != nil {
			log.Printf("%s: Writing feed failed: %v", w.pod, err)
		} else {
			lastTimeAnyFeedWritten.Set(time.Now())
		}
	}

	if w.initialCheck {
		// This is helpful to appear in the log (but only once per podcast).
		log.Printf("%s: URL for feed is configured as %s",
			w.pod, w.cfg.urlFor(w.pod.feedPath()))
	}
}

func (w *watcher) download(vi ytVidInfo, firstTry bool) error {
	diskPath := vi.episodePath(w.cfg.YTDLWriteExt)
	if _, err := os.Stat(diskPath); err == nil {
		return nil
	}

	cmdLine := fmt.Sprintf("%s -f %s -o %s --socket-timeout 30 -- %s",
		downloadCmdName, w.cfg.YTDLFmtSelector, diskPath, vi.id)
	if firstTry {
		log.Printf("%s: Download intent: %s", w.pod, cmdLine)
	}

	var errBuf bytes.Buffer
	cmdLineSplit := strings.Split(cmdLine, " ")
	cmd := exec.Command(cmdLineSplit[0], cmdLineSplit[1:]...)
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("%v: %s", err, errBuf.String())
	}
	return err
}

func (w *watcher) writeFeed() error {
	// Construct the blurb used in the feed description that's likely displayed
	// to podcast client users.
	feedDesc := new(bytes.Buffer)
	feedDesc.WriteString(w.pod.Description)
	if feedDesc.Len() == 0 {
		// No custom description was provided in the config. Derive one from
		// the rest of the config.
		fmt.Fprint(feedDesc,
			"Generated based on the videos of YouTube channel ",
			w.pod.YTChannelReadableName)
		if !w.pod.Epoch.IsZero() {
			fmt.Fprintf(feedDesc, " published from %s onwards",
				w.pod.EpochStr)
		}
		if w.pod.TitleFilterStr != "" {
			fmt.Fprintf(feedDesc, " with titles matching \"%s\"",
				w.pod.TitleFilterStr)
		}
	}

	// Use the podcasts package to construct the XML for the file.
	feedBuilder := &podcasts.Podcast{
		Title:       w.pod.Name,
		Link:        "https://www.youtube.com/channel/" + w.pod.YTChannelID,
		Copyright:   w.pod.YTChannelReadableName,
		Language:    "en",
		Generator:   versionLabel + " from https://github.com/frou/yt2pod",
		Description: feedDesc.String(),
	}

	// Sort so that episodes in the feed are ordered newest to oldest.
	sort.Sort(sort.Reverse(vidsChronoSorter(w.vids)))

	for _, vi := range w.vids {
		diskPath := vi.episodePath(w.cfg.YTDLWriteExt)
		f, err := os.Open(diskPath)
		if err != nil {
			log.Print(err)
			continue
		}
		info, err := f.Stat()
		f.Close()
		if err != nil {
			log.Print(err)
			continue
		}
		epSize := info.Size()
		epURL := w.cfg.urlFor(diskPath)
		epSummary := &podcasts.ItunesSummary{Value: fmt.Sprintf(
			`%s // <a href="https://www.youtube.com/watch?v=%s">`+
				`Link to original YouTube video</a>`,
			vi.desc, vi.id)}

		enclosureType := "audio"
		if w.pod.Vidya {
			enclosureType = "video"
		}
		enclosureType = fmt.Sprint(enclosureType, "/", w.cfg.YTDLWriteExt)

		feedBuilder.AddItem(&podcasts.Item{
			Title:   vi.title,
			Summary: epSummary,
			GUID:    epURL,
			PubDate: &podcasts.PubDate{Time: vi.published},
			Enclosure: &podcasts.Enclosure{
				URL:    epURL,
				Length: fmt.Sprint(epSize),
				Type:   enclosureType,
			},
		})
	}
	feed, err := feedBuilder.Feed(
		// Apply iTunes-specific XML elements.
		podcasts.Author(feedBuilder.Copyright),
		podcasts.Summary(feedBuilder.Description),
		podcasts.Image(w.cfg.urlFor(w.pod.artPath())))
	if err != nil {
		return err
	}

	// Write the feed XML to disk.
	f, err := os.OpenFile(w.pod.feedPath(),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, stdext.OwnerWritableReg)
	if err != nil {
		return err
	}
	defer f.Close()
	log.Printf("%s: Writing out feed", w.pod)
	if err := feed.Write(f); err != nil {
		return err
	}
	fmt.Fprintln(f)
	return nil
}

func (w *watcher) getLatest(pubdAfter time.Time) ([]ytVidInfo, error) {
	var (
		latestVids    []ytVidInfo
		nextPageToken string
	)
	for {
		apiReq := w.ytAPI.Search.List("id,snippet").
			ChannelId(w.pod.YTChannelID).
			Type("video").
			PublishedAfter(pubdAfter.Format(time.RFC3339)).
			Order("date").
			MaxResults(50).
			PageToken(nextPageToken)
		checkTime := time.Now()
		apiResp, err := apiReq.Do()
		if err != nil {
			// Don't hammer on the API if it's down or isn't happy.
			w.ytAPIRespite = ytAPIRespiteUnit
			return nil, err
		}
		w.lastChecked = checkTime
		for _, item := range apiResp.Items {
			if item.Id.Kind != "youtube#video" {
				return nil, errors.New("non-video in response items")
			}
			if !w.pod.TitleFilter.MatchString(item.Snippet.Title) {
				// Not interested in this vid.
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

var ytChannelIDFormat = regexp.MustCompile("UC[[:alnum:]_-]{22}")

func (w *watcher) getChannelInfo() error {
	apiReq := w.ytAPI.Channels.List("id,snippet").MaxResults(1)
	// Work out whether yt_channel specified in config is an ID or Username and
	// modify the API request accordingly.
	if ytChannelIDFormat.MatchString(w.pod.YTChannel) {
		apiReq = apiReq.Id(w.pod.YTChannel)
	} else {
		apiReq = apiReq.ForUsername(w.pod.YTChannel)
	}
	apiResp, err := apiReq.Do()
	if err != nil {
		return err
	}

	switch len(apiResp.Items) {
	case 0:
		return errors.New("not a channel id: " + w.pod.YTChannelID)
	case 1:
		if apiResp.Items[0].Kind == "youtube#channel" {
			break
		}
		fallthrough
	default:
		return errors.New("expected exactly 1 channel in response items")
	}
	ch := apiResp.Items[0]

	// We now know the channel's ID regardless of whether it was in config.
	w.pod.YTChannelID = ch.Id
	w.pod.YTChannelReadableName = ch.Snippet.Title

	var chImg image.Image
	if w.pod.CustomImagePath == "" {
		// Get the channel image referenced by the thumbnail field.
		chImgURL := ch.Snippet.Thumbnails.High.Url
		chImgResp, err := http.Get(chImgURL)
		if err != nil {
			return err
		}
		defer chImgResp.Body.Close()

		switch typ := chImgResp.Header.Get("Content-Type"); typ {
		case "image/jpeg":
			chImg, err = jpeg.Decode(chImgResp.Body)
		case "image/png":
			chImg, err = png.Decode(chImgResp.Body)
		default:
			err = fmt.Errorf("%s: channel image: unexpected type: %s",
				w.pod, typ)
		}
		if err != nil {
			return err
		}
	} else {
		if w.pod.CustomImagePath == w.pod.artPath() {
			return fmt.Errorf(
				"%s: custom image path and automatic image path clash "+
					"(both are: %v)", w.pod, w.pod.CustomImagePath)
		}
		f, err := os.Open(w.pod.CustomImagePath)
		if err != nil {
			return fmt.Errorf("%s: custom image: %v", w.pod, err)
		}
		defer f.Close()
		chImg, _, err = image.Decode(f)
		if err != nil {
			return fmt.Errorf("%s: custom image: %v", w.pod, err)
		}
		log.Printf("%s: Using custom image from path %s",
			w.pod, w.pod.CustomImagePath)
	}

	// Ensure that the dimensions of the image meet the minimum requirements to
	// be listed in the iTunes podcast directory.
	width, height := chImg.Bounds().Max.X, chImg.Bounds().Max.Y
	const minDim = 1400
	if width < minDim || height < minDim {
		var rw, rh uint
		// The smaller dim must meet the minimum. Other than that, keep aspect.
		if height < width {
			rw, rh = 0, minDim
		} else {
			rw, rh = minDim, 0
		}
		chImg = resize.Resize(rw, rh, chImg, resize.Bicubic)
	}

	// Write the image to disk.
	f, err := os.OpenFile(w.pod.artPath(),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, stdext.OwnerWritableReg)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, chImg, nil)
}
