package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/frou/poor-mans-generics/set"
	"github.com/go-playground/validator"
)

type config struct {
	// High-level
	YTDataAPIKey           string    `json:"yt_data_api_key"          validate:"required"`
	Podcasts               []podcast `json:"podcasts"                 validate:"required,dive"`
	ServeHost              string    `json:"serve_host"               validate:"hostname"`
	ServePort              int       `json:"serve_port"               validate:"min=1,max=65535"`
	ServeDirectoryListings bool      `json:"serve_directory_listings" validate:"-"`
	LinkProxy              string    `json:"link_proxy"               validate:"omitempty,uri"`

	// Watcher-related
	CheckIntervalMinutes int    `json:"check_interval_minutes" validate:"min=1"`
	YTDLFmtSelector      string `json:"ytdl_fmt_selector"      validate:"required"`
	YTDLWriteExt         string `json:"ytdl_write_ext"         validate:"alphanum"`
}

// ------------------------------------------------------------

type podcast struct {
	YTChannel             string `json:"yt_channel" validate:"required"`
	YTChannelID           string
	YTChannelReadableName string

	Name        string `json:"name"        validate:"required"`
	ShortName   string `json:"short_name"  validate:"required"`
	Description string `json:"description" validate:"-"`

	TitleFilterStr string `json:"title_filter" validate:"-"`
	TitleFilter    *regexp.Regexp

	EpochStr string `json:"epoch" validate:"epochformat"`
	Epoch    time.Time

	Vidya           bool   `json:"vidya" validate:"-"`
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
		re, err := regexp.Compile(
			// Ensure the re does case-insensitive matching.
			fmt.Sprintf("(?i:%s)", c.Podcasts[i].TitleFilterStr))
		if err != nil {
			return nil, err
		}
		c.Podcasts[i].TitleFilter = re
		// @todo Detect whether the TitleFilter is entirely a literal pattern
		// @body ...in which case the title-based filtering can be done server-side using the `q` param: https://developers.google.com/youtube/v3/docs/search/list#q
		// @body and this will save API calls vs. fetching all result pages and doing the title filtering client-side.
		// @body Use https://pkg.go.dev/regexp#Regexp.LiteralPrefix ?
	}

	return c, err
}

func initValidator() *validator.Validate {
	validate := validator.New()

	epochDateRE := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})?$`)
	validate.RegisterValidation("epochformat", func(fl validator.FieldLevel) bool {
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
		c := sl.Current().Interface().(config)
		var podcastShortNameSet set.Strings
		for i := range c.Podcasts {
			sn := c.Podcasts[i].ShortName
			if podcastShortNameSet.Contains(sn) {
				sl.ReportError(
					c.Podcasts,
					fmt.Sprintf("podcasts[%d].short_name", i),
					"",
					fmt.Sprintf("Multiple podcasts are using the same %q short_name", sn),
					"")
				continue
			}
			podcastShortNameSet.Add(sn)
		}
	}, config{})

	return validate
}
