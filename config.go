package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"
)

func loadConfig(path string) (*config, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c config
	if err := json.Unmarshal(buf, &c); err != nil {
		return nil, err
	}
	shortNameSet := make(map[string]struct{})
	for i := range c.Shows {
		// Parse Epoch
		t, err := time.Parse("2006-01-02", c.Shows[i].EpochStr)
		if err != nil {
			return nil, err
		}
		c.Shows[i].Epoch = t

		// Parse Title Filter
		re, err := regexp.Compile(c.Shows[i].TitleFilterStr)
		if err != nil {
			return nil, err
		}
		c.Shows[i].TitleFilter = re

		sn := c.Shows[i].ShortName
		if _, found := shortNameSet[sn]; found {
			return nil, fmt.Errorf("multiple shows using shortname \"%s\"", sn)
		}
		shortNameSet[sn] = struct{}{}
	}
	return &c, err
}

// ------------------------------------------------------------

type config struct {
	YTDataAPIKey         string `json:"yt_data_api_key"`
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
	DownloadAudioFormat  string `json:"audio_format"`
	ServeHost            string `json:"serve_host"`
	ServePort            int    `json:"serve_port"`
	Shows                []show `json:"shows"`
}

func (c *config) servingLink(resource string) string {
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
	return filepath.Join(metaSubdir, s.ShortName+".xml")
}

func (s *show) artPath() string {
	return filepath.Join(metaSubdir, s.ShortName+".jpg")
}

func (s *show) String() string {
	return s.ShortName
}
