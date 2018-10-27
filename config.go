package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/frou/poor-mans-generics/set"
)

type config struct {
	// High-level
	YTDataAPIKey           string    `json:"yt_data_api_key"          valid:"required"`
	Podcasts               []podcast `json:"podcasts"                 valid:"required"`
	ServeHost              string    `json:"serve_host"               valid:"host"`
	ServePort              int       `json:"serve_port"               valid:"port"`
	ServeDirectoryListings bool      `json:"serve_directory_listings" valid:"-"`

	// Watcher-related
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
	YTDLFmtSelector      string `json:"ytdl_fmt_selector"`
	YTDLWriteExt         string `json:"ytdl_write_ext"`
}

// ------------------------------------------------------------

type podcast struct {
	YTChannel             string `json:"yt_channel"`
	YTChannelID           string
	YTChannelReadableName string

	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	Description string `json:"description"`

	TitleFilterStr string `json:"title_filter"`
	TitleFilter    *regexp.Regexp

	EpochStr string `json:"epoch"`
	Epoch    time.Time

	Vidya           bool   `json:"vidya"`
	CustomImagePath string `json:"custom_image"`
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

	if _, err := govalidator.ValidateStruct(c); err != nil {
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
