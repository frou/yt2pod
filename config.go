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
)

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

	// Do some sanity checks on it.
	if c.YTDataAPIKey == "" {
		return nil, errors.New("missing YouTube Data API key")
	}
	if min := 1; c.CheckIntervalMinutes < min {
		return nil, fmt.Errorf("check interval must be >= %d minutes", min)
	}
	c.YTDLWriteExt = strings.Trim(c.YTDLWriteExt, ".")
	shortNameSet := make(map[string]struct{})
	for i := range c.Shows {
		// Parse Epoch
		var t time.Time
		var err error
		if es := c.Shows[i].EpochStr; es != "" {
			t, err = time.Parse("2006-01-02", es)
			if err != nil {
				return nil, err
			}
		}
		c.Shows[i].Epoch = t

		// Parse Title Filter
		re, err := regexp.Compile(
			// Ensure the re does case-insensitive matching.
			fmt.Sprintf("(?i:%s)", c.Shows[i].TitleFilterStr))
		if err != nil {
			return nil, err
		}
		c.Shows[i].TitleFilter = re

		// Check for show shortname (in effect primary key) collisions.
		sn := c.Shows[i].ShortName
		if _, found := shortNameSet[sn]; found {
			return nil, fmt.Errorf("multiple shows using shortname \"%s\"", sn)
		}
		shortNameSet[sn] = struct{}{}
	}

	return c, err
}

// ------------------------------------------------------------

type config struct {
	YTDataAPIKey         string `json:"yt_data_api_key"`
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
	YTDLFmtSelector      string `json:"ytdl_fmt_selector"`
	YTDLWriteExt         string `json:"ytdl_write_ext"`
	ServeHost            string `json:"serve_host"`
	ServePort            int    `json:"serve_port"`
	Shows                []show `json:"shows"`
}

func (c *config) urlFor(resource string) string {
	return fmt.Sprintf("http://%s:%d/%s", c.ServeHost, c.ServePort, resource)
}

// ------------------------------------------------------------

type show struct {
	YTChannelID           string `json:"yt_channel_id"`
	YTReadableChannelName string

	Name      string `json:"name"`
	ShortName string `json:"short_name"`

	TitleFilterStr string `json:"title_filter"`
	TitleFilter    *regexp.Regexp

	EpochStr string `json:"epoch"`
	Epoch    time.Time
}

func (s *show) feedPath() string {
	return filepath.Join(dataSubdirMetadata, s.ShortName+".xml")
}

func (s *show) artPath() string {
	return filepath.Join(dataSubdirMetadata, s.ShortName+".jpg")
}

func (s *show) String() string {
	return s.ShortName
}
