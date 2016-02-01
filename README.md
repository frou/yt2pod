[yt2pod] is a daemon that monitors YouTube channels and publishes audio
podcasts of them.

A podcast episode needn't be created for every video uploaded to a channel.
They can be filtered based on title and upload date. A single instance of
yt2pod can monitor multiple YouTube channels simultaneously and publish a
separate audio podcast based on each (and even multiple podcasts based on the
same one).

A built-in webserver serves the following for each podcast.
* RSS Feed: `/meta/{configured_short_name}.xml`
* Artwork: `/meta/{configured_short_name}.jpg`
* Audio Episodes: `/ep/{yt_video_id}.{configured_file_ext}`

---

# Configuration

Main configuration is done using a [JSON file][egcfg].
For each podcast ("show"):

* `title_filter` is a regexp (always case-insensitive)

* `epoch` is a date (YYYY-MM-DD or blank to mean from-the-beginning)

* `yt_channel_id` is a ~24 character string identifying the YouTube channel
(different from its readable username). When at a channel's page in your web
browser, if the ID is not shown as part of the URL, you can still find it by
searching the page source for the first instance of `data-channel-external-id=`

## YouTube Data API

YouTube's Data API is used to query information. If you want to use [your
own][apikey] API key, replace the one in the example config.

## Flags

There are some command-line flags:

```text
$ yt2pod -help

usage:
  yt2pod [flags]

flags:
  -config string
        path to config file (default "config.json")
  -data string
        path to directory to change into and write data (created if needed) (default "data")
  -syslog
        send log statements to syslog rather than writing them to stderr
  -version
        show version information then exit
```


# Building & Running

With [Go] installed (available in all good package managers):

`$ go get github.com/frou/yt2pod`

The `yt2pod` binary should now be built and located in `$GOPATH/bin/`

The only runtime depenency is the [youtube-dl command][ytdl] (available in all
good package managers).

---

Copyright (c) 2015 Duncan Holm



[yt2pod]: https://github.com/frou/yt2pod
[egcfg]: https://github.com/frou/yt2pod/blob/master/example_config.json
[ytdl]: https://rg3.github.io/youtube-dl/
[apikey]: https://developers.google.com/youtube/registering_an_application
[go]: https://golang.org/
