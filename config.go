package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/zyedidia/generic/mapset"
)

type config struct {
	// High-level
	YTDataAPIKey           string    `json:"yt_data_api_key"          validate:"required"`
	Podcasts               []podcast `json:"podcasts"                 validate:"required,dive"`
	ServeHost              string    `json:"serve_host"               validate:"hostname"`
	ServePort              int       `json:"serve_port"               validate:"min=1,max=65535"`
	ServeDirectoryListings bool      `json:"serve_directory_listings" validate:"-"`
	LinkProxy              string    `json:"link_proxy"               validate:"omitempty,uri"`
	DownloaderName         string    `json:"downloader_name"          validate:"-"`

	// Watcher-related
	CheckIntervalMinutes int    `json:"check_interval_minutes"  validate:"min=1"`
	YTDLFmtSelector      string `json:"ytdl_fmt_selector"       validate:"required"`
	YTDLWriteExt         string `json:"ytdl_write_ext"          validate:"alphanum"`
	YTDLVideoFmtSelector string `json:"ytdl_video_fmt_selector" validate:"required"`
	YTDLVideoWriteExt    string `json:"ytdl_video_write_ext"    validate:"alphanum"`
}

// ------------------------------------------------------------

type podcast struct {
	YTChannelHandle       string `json:"yt_channel" validate:"required"`
	YTChannelHandleFormat channelHandleFormat
	YTChannelID           string
	YTChannelReadableName string

	Name        string `json:"name"        validate:"required"`
	ShortName   string `json:"short_name"  validate:"required"`
	Description string `json:"description" validate:"-"`

	TitleFilter          string `json:"title_filter" validate:"-"`
	TitleFilterIsLiteral bool
	TitleFilterRE        *regexp.Regexp

	EpochStr string `json:"epoch" validate:"epochformat"`
	Epoch    time.Time

	Video           bool   `json:"video" validate:"-"`
	CustomImagePath string `json:"custom_image" validate:"-"`
}

func (p *podcast) feedPath() string {
	return filepath.Join(dataSubdirMetadata, p.ShortName+".xml")
}

func (p *podcast) artPath() string {
	return filepath.Join(dataSubdirMetadata, p.ShortName+".jpg")
}

func (p *podcast) String() string {
	return p.ShortName
}

// ------------------------------------------------------------

type channelHandleFormat int

// REF: https://support.google.com/youtube/answer/6180214
const (
	LegacyUsername channelHandleFormat = iota
	ChannelID
	// @todo Support "Custom URL" channel identifiers in addition to Channel-IDs and Usernames
	// @body https://stackoverflow.com/questions/37267324/how-to-get-youtube-channel-details-using-youtube-data-api-if-channel-has-custom
	// @body https://github.com/mattwright324/youtube-metadata/issues/1
	// @body https://stackoverflow.com/questions/68490562/youtube-channel-with-custom-url-but-no-c-slug-in-url-does-not-return-snippet
	//CustomUrl
	// @todo Yet another possibility has been introduced: @handles
	// @body https://support.google.com/youtube/answer/6180214
	// @body https://support.google.com/youtube/answer/11585688
)

const (
	youtubeHomeUrl = "https://www.youtube.com"

	youtubeChannelUrlPrefix = youtubeHomeUrl + "/channel/"
	youtubeUserUrlPrefix    = youtubeHomeUrl + "/user/"
	// youtubeCustomUrlPrefix  = youtubeHomeUrl + "/c/"
)

// ------------------------------------------------------------

//nolint:gocognit
func loadConfig(path string) (c *config, err error) {
	// Load & decode config from disk.
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c = new(config)
	if err := json.Unmarshal(buf, c); err != nil {
		return nil, err
	}
	validate := initValidator()
	if err := validate.Struct(c); err != nil {
		return nil, err
	}

	for i := range c.Podcasts {
		handle := c.Podcasts[i].YTChannelHandle
		switch {
		case strings.HasPrefix(handle, youtubeUserUrlPrefix):
			c.Podcasts[i].YTChannelHandle = strings.TrimPrefix(handle, youtubeUserUrlPrefix)
			c.Podcasts[i].YTChannelHandleFormat = LegacyUsername
		case strings.HasPrefix(handle, youtubeChannelUrlPrefix) || ytChannelIDFormat.MatchString(handle):
			c.Podcasts[i].YTChannelHandle = strings.TrimPrefix(handle, youtubeChannelUrlPrefix)
			c.Podcasts[i].YTChannelHandleFormat = ChannelID
		// case strings.HasPrefix(handle, youtubeCustomUrlPrefix):
		// 	c.Podcasts[i].YTChannelHandle = strings.TrimPrefix(handle, youtubeCustomUrlPrefix)
		// 	c.Podcasts[i].YTChannelHandleFormat = CustomUrl
		default:
			log.Printf("Assuming that channel handle %q in config is a legacy YouTube username", handle)
			c.Podcasts[i].YTChannelHandleFormat = LegacyUsername
		}

		// Parse Epoch
		var t time.Time
		var err error
		if es := c.Podcasts[i].EpochStr; es != "" {
			t, err = time.Parse("2006-01-02", es)
			if err != nil {
				return nil, err
			}
		}
		c.Podcasts[i].Epoch = t

		// Parse Title Filter
		re, err := regexp.Compile(c.Podcasts[i].TitleFilter)
		if err != nil {
			return nil, fmt.Errorf("error in regex specified for title filter: %w", err)
		}
		_, c.Podcasts[i].TitleFilterIsLiteral = re.LiteralPrefix()
		if !c.Podcasts[i].TitleFilterIsLiteral {
			log.Printf("Warning: title filter for %q contains regexp metacharacters so may cause high YouTube API quota usage", c.Podcasts[i].Name)
		}
		// Force case-insensitive matching.
		c.Podcasts[i].TitleFilterRE = regexp.MustCompile(fmt.Sprintf("(?i:%s)", re.String()))
	}

	// Listed in descending priority
	defaultDownloaderNames := []string{
		// REF: https://github.com/yt-dlp/yt-dlp
		"yt-dlp",
		// REF: https://github.com/ytdl-org/youtube-dl
		"youtube-dl",
	}
	if c.DownloaderName == "" {
		for _, candidateName := range defaultDownloaderNames {
			if _, err := exec.LookPath(candidateName); err != nil {
				continue
			}
			c.DownloaderName = candidateName
			break
		}
	}
	if c.DownloaderName == "" {
		return nil, fmt.Errorf("No downloader command is available. Please install one of %v and ensure it's on PATH", defaultDownloaderNames)
	}

	return c, err
}

func initValidator() *validator.Validate {
	validate := validator.New()

	epochDateRE := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})?$`)
	_ = validate.RegisterValidation("epochformat", func(fl validator.FieldLevel) bool {
		return epochDateRE.MatchString(fl.Field().String())
	})

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	validate.RegisterStructValidation(func(sl validator.StructLevel) {
		c, _ := sl.Current().Interface().(config)
		podcastShortNameSet := mapset.New[string]()
		for i := range c.Podcasts {
			sn := c.Podcasts[i].ShortName
			if podcastShortNameSet.Has(sn) {
				sl.ReportError(
					c.Podcasts,
					fmt.Sprintf("podcasts[%d].short_name", i),
					"",
					fmt.Sprintf("Can't use %q because another configured podcast already uses it", sn),
					"")
				continue
			}
			podcastShortNameSet.Put(sn)
		}
	}, config{})

	return validate
}
