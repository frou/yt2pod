package main

import (
	"bytes"
	_ "embed"
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

	"github.com/jbub/podcasts"
	"github.com/nfnt/resize"
	"google.golang.org/api/youtube/v3"

	"github.com/frou/stdext"
)

//go:embed placeholder-art.png
var placeholderArtPNG []byte

type watcher struct {
	ytAPI         *youtube.Service
	cfg           *config
	pod           *podcast
	checkInterval time.Duration

	initialCheck bool
	lastChecked  time.Time
	ytAPIRespite time.Duration
	vids         []ytVidInfo

	problemVids map[string]ytVidInfo
	cleanc      chan *cleaningWhitelist
}

func newWatcher(
	ytAPI *youtube.Service,
	cfg *config,
	pod *podcast,
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
			w.pod, w.buildURL(w.pod.feedPath()))
	}
}

func (w *watcher) formatSelector() string {
	if w.pod.Video {
		return w.cfg.YTDLVideoFmtSelector
	} else {
		return w.cfg.YTDLFmtSelector
	}
}

func (w *watcher) fileExtension() string {
	if w.pod.Video {
		return w.cfg.YTDLVideoWriteExt
	} else {
		return w.cfg.YTDLWriteExt
	}
}

func (w *watcher) download(vi ytVidInfo, firstTry bool) error {
	diskPath := vi.episodePath(w.fileExtension())
	if _, err := os.Stat(diskPath); err == nil {
		return nil
	}

	cmdLine := fmt.Sprintf("%s -f %s -o %s --socket-timeout 30 -- %s",
		downloadCmdName, w.formatSelector(), diskPath, vi.id)
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

func (w *watcher) buildURL(filePath string) string {
	var portPart string
	if w.cfg.ServePort != 80 {
		portPart = fmt.Sprintf(":%d", w.cfg.ServePort)
	}

	if w.cfg.LinkProxy != "" {
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(w.cfg.LinkProxy, "/"), filePath)
	} else {
		return fmt.Sprintf("http://%s%s/%s", w.cfg.ServeHost, portPart, filePath)
	}
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
			fmt.Fprintf(feedDesc, " published from %s onwards", w.pod.EpochStr)
		}
		if w.pod.TitleFilter != "" {
			fmt.Fprintf(feedDesc, " with titles matching \"%s\"", w.pod.TitleFilter)
		}
	}

	// Use the podcasts package to construct the XML for the file.
	var homeLink string
	switch w.pod.YTChannelHandleFormat {
	case LegacyUsername:
		homeLink = youtubeUserUrlPrefix + w.pod.YTChannelHandle
	case ChannelID:
		homeLink = youtubeChannelUrlPrefix + w.pod.YTChannelHandle
	}
	feedBuilder := &podcasts.Podcast{
		Title:       w.pod.Name,
		Link:        homeLink,
		Copyright:   w.pod.YTChannelReadableName,
		Language:    "en",
		Description: feedDesc.String(),
	}

	// Sort so that episodes in the feed are ordered newest to oldest.
	sort.Sort(sort.Reverse(vidsChronoSorter(w.vids)))

	for _, vi := range w.vids {
		diskPath := vi.episodePath(w.fileExtension())
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
		epURL := w.buildURL(diskPath)
		epSummary := &podcasts.ItunesSummary{Value: fmt.Sprintf(
			`%s // <a href="%s/watch?v=%s">Link to original YouTube video</a>`,
			vi.desc,
			youtubeHomeUrl,
			vi.id),
		}

		enclosureType := "audio"
		if w.pod.Video {
			enclosureType = "video"
		}
		enclosureType = fmt.Sprint(enclosureType, "/", w.fileExtension())

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
		podcasts.Image(w.buildURL(w.pod.artPath())))
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
		if w.pod.TitleFilterIsLiteral && w.pod.TitleFilter != "" {
			// When the user-specified title filter is a plain literal
			// (doesn't use any regex syntax) then filtering can be done
			// server-side. This can save on API quota usage by reducing the
			// number of pages of results that need to be requested.
			apiReq = apiReq.Q(w.pod.TitleFilter)
		}
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
			// Even if we requested server-side filtering, that is fuzzy and
			// often returns false-positives, so we always do client-side
			// filtering.
			if !w.pod.TitleFilterRE.MatchString(item.Snippet.Title) {
				// Not interested in this vid.
				continue
			}
			pubd, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			if err != nil {
				return nil, err
			}
			latestVids = append(
				latestVids,
				makeYtVidInfo(item.Id.VideoId, pubd, item.Snippet.Title, item.Snippet.Description))
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

	switch w.pod.YTChannelHandleFormat {
	case LegacyUsername:
		apiReq = apiReq.ForUsername(w.pod.YTChannelHandle)
	case ChannelID:
		apiReq = apiReq.Id(w.pod.YTChannelHandle)
	}

	var channel *youtube.Channel
	apiResp, err := apiReq.Do()
	if err != nil {
		log.Printf("%s: Getting initial channel info failed: %v", w.pod, err)
	} else {
		switch n := len(apiResp.Items); n {
		case 0:
			return fmt.Errorf("%s: could not find a channel by using the handle %q", w.pod, w.pod.YTChannelHandle)
		case 1:
			if item := apiResp.Items[0]; item.Kind == "youtube#channel" {
				channel = item
			} else {
				return fmt.Errorf("%s: unexpected Kind %q in initial channel info", w.pod, item.Kind)
			}
		default:
			return fmt.Errorf("%s: expected exactly 1 item in initial channel info response, got %d", w.pod, n)
		}
	}

	if channel != nil {
		// We have now been made aware of the channel's ChannelID regardless of
		// what format of handle was specified in the config file.
		w.pod.YTChannelID = channel.Id

		w.pod.YTChannelReadableName = channel.Snippet.Title
	} else {
		if w.pod.YTChannelHandleFormat == ChannelID {
			w.pod.YTChannelID = w.pod.YTChannelHandle
		} else {
			return fmt.Errorf(
				"%s: Cannot continue due to being unable to discover the ChannelID for %q using the YT API. HINT: In the config file, writing the channel's ID (\"UC...\") directly instead of %[2]q will resolve this",
				w.pod, w.pod.YTChannelHandle)
		}
		log.Printf("%s: Unable to discover channel's readable name using the YT API, so making do with the handle from the config file", w.pod)
		w.pod.YTChannelReadableName = w.pod.YTChannelHandle
	}

	chImg, err := w.getChannelImage(channel)
	if err != nil {
		return err
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

func (w *watcher) getChannelImage(channel *youtube.Channel) (image.Image, error) {
	if w.pod.CustomImagePath != "" {
		if w.pod.CustomImagePath == w.pod.artPath() {
			return nil, fmt.Errorf(
				"%s: custom image path and automatic image path clash "+
					"(both are: %v)", w.pod, w.pod.CustomImagePath)
		}
		f, err := os.Open(w.pod.CustomImagePath)
		if err != nil {
			return nil, fmt.Errorf("%s: custom image: %v", w.pod, err)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("%s: custom image: %v", w.pod, err)
		}
		log.Printf("%s: Using custom image from path %s", w.pod, w.pod.CustomImagePath)
		return img, nil
	} else {
		if channel == nil {
			log.Printf("%s: Unable to discover channel's image using the YT API, so making do with placeholder art", w.pod)
			return png.Decode(bytes.NewReader(placeholderArtPNG))
		}

		imgResp, err := http.Get(channel.Snippet.Thumbnails.High.Url)
		if err != nil {
			return nil, err
		}
		defer imgResp.Body.Close()

		switch typ := imgResp.Header.Get("Content-Type"); typ {
		case "image/jpeg":
			return jpeg.Decode(imgResp.Body)
		case "image/png":
			return png.Decode(imgResp.Body)
		default:
			return nil, fmt.Errorf("%s: channel image: unexpected type: %s", w.pod, typ)
		}
	}
}
