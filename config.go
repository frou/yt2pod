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
	"gopkg.in/go-playground/validator.v9"
)

type config struct {
	// High-level
	YTDataAPIKey           string    `json:"yt_data_api_key"          validate:"required"`
	Podcasts               []podcast `json:"podcasts"                 validate:"required,dive"`
	ServeHost              string    `json:"serve_host"               validate:"hostname"`
	ServePort              int       `json:"serve_port"               validate:"min=1,max=65535"`
	ServeDirectoryListings bool      `json:"serve_directory_listings" validate:"-"`

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

	EpochStr string `json:"epoch"`
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

	validate := validator.New()
	epochDateRE := regexp.MustCompile(`^([[:digit:]]{4}-[[:digit:]]{2}-[[:digit:]]{2})?$`)
	validate.RegisterValidation("epochdate", func(fl validator.FieldLevel) bool {
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
					fmt.Sprintf("Podcasts[%d].ShortName", i),
					"",
					fmt.Sprintf("Multiple podcasts are using the same short name %q", sn),
					"")
				continue
			}
			podcastShortNameSet.Add(sn)
		}
	}, config{})
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
	}

	return c, err
}
