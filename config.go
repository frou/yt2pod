package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"github.com/frou/poor-mans-generics/set"
	"gopkg.in/validator.v2"
)

type config struct {
	// High-level
	YTDataAPIKey           string    `json:"yt_data_api_key"          validate:"nonzero"`
	Podcasts               []podcast `json:"podcasts"                 validate:""`
	ServeHost              string    `json:"serve_host"               validate:"nonzero"`
	ServePort              int       `json:"serve_port"               validate:"min=1,max=65535"`
	ServeDirectoryListings bool      `json:"serve_directory_listings" validate:""`

	// Watcher-related
	CheckIntervalMinutes int    `json:"check_interval_minutes" validate:"min=1"`
	YTDLFmtSelector      string `json:"ytdl_fmt_selector"      validate:"nonzero"`
	YTDLWriteExt         string `json:"ytdl_write_ext"         validate:"regexp=^[[:alnum:]]+$"`
}

// ------------------------------------------------------------

type podcast struct {
	YTChannel             string `json:"yt_channel" validate:"nonzero"`
	YTChannelID           string
	YTChannelReadableName string

	Name        string `json:"name"         validate:"nonzero"`
	ShortName   string `json:"short_name"   validate:"nonzero"`
	Description string `json:"description"  validate:""`

	TitleFilterStr string `json:"title_filter" validate:""`
	TitleFilter    *regexp.Regexp

	EpochStr string `json:"epoch" validate:"regexp=^([[:digit:]]{4}-[[:digit:]]{2}-[[:digit:]]{2})?$"`
	Epoch    time.Time

	Vidya           bool   `json:"vidya"        validate:""`
	CustomImagePath string `json:"custom_image" validate:""`
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
	if err := validator.Validate(c); err != nil {
		return nil, err
	}

	var podcastShortNameSet set.Strings
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

		// Check for podcast shortname (in effect primary key) collisions.
		sn := c.Podcasts[i].ShortName
		if podcastShortNameSet.Contains(sn) {
			return nil, fmt.Errorf(
				"multiple podcasts using shortname \"%s\"", sn)
		}
		podcastShortNameSet.Add(sn)
	}

	return c, err
}
