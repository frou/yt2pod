package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/frou/poor-mans-generics/set"
)

type config struct {
	// High-level
	YTDataAPIKey           string    `json:"yt_data_api_key"`
	Podcasts               []podcast `json:"podcasts"`
	ServeHost              string    `json:"serve_host"`
	ServePort              int       `json:"serve_port"`
	ServeDirectoryListings bool      `json:"serve_directory_listings"`

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

	// Do some sanity checks the loaded values:

	if c.YTDataAPIKey == "" {
		return nil, errors.New("missing YouTube Data API key")
	}
	if min := 1; c.CheckIntervalMinutes < min {
		return nil, fmt.Errorf("check interval must be >= %d minutes", min)
	}
	if c.YTDLFmtSelector == "" {
		return nil, fmt.Errorf("missing %s format selector", downloadCmdName)
	}
	if c.YTDLWriteExt == "" {
		return nil, fmt.Errorf("missing %s file type extension",
			downloadCmdName)
	}
	if c.ServeHost == "" {
		return nil, errors.New("missing host to webserve on")
	}
	if c.ServePort == 0 {
		return nil, errors.New("missing fixed port to webserve on")
	}

	// Normalize e.g. ".m4a" and "m4a"
	c.YTDLWriteExt = strings.TrimLeft(c.YTDLWriteExt, ".")

	if len(c.Podcasts) == 0 {
		return nil, errors.New("no podcasts are defined")
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
		// TODO: Check that shortname is not empty string either
		if podcastShortNameSet.Contains(sn) {
			return nil, fmt.Errorf(
				"multiple podcasts using shortname \"%s\"", sn)
		}
		podcastShortNameSet.Add(sn)
	}

	return c, err
}
