[yt2pod] is a server daemon that monitors YouTube and publishes an audio
podcast based on a channel's videos.

A podcast episode needn't be created for every video uploaded. They can be
filtered based on title and upload date. A single instance of yt2pod can
monitor multiple YouTube channels simultaneously and publish a separate audio
podcast based on each (and even multiple podcasts based on the same channel).

---

Main configuration is done using a [JSON file][egcfg]. In it, `title_filter` is
a regexp (always case-insensitive) and `epoch` is a date (YYYY-MM-DD or blank
to mean from-the-beginning).

In addition, there are some command-line flags:

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

A built-in webserver serves the following files for each podcast.
* `xml` RSS feed
* `jpg` artwork
* `m4a` audio episodes

# Building

With [Go] installed (available in all good package managers):

`$ go get github.com/frou/yt2pod`

The `yt2pod` binary should now be built and located in `$GOPATH/bin/`

The only runtime depenency is the [youtube-dl command][ytdl] (available in all
good package managers).

# YouTube Data API

YouTube's Data API is used to query information. If you do not already have an
API key, [get one from Google for free][apikey] (ignore the OAuth stuff - a
basic server key is what's needed). Put your API key in your
[config file][egcfg].

---

Copyright (c) 2015 Duncan Holm



[yt2pod]: https://github.com/frou/yt2pod
[egcfg]: https://github.com/frou/yt2pod/blob/master/example_config.json
[ytdl]: https://github.com/rg3/youtube-dl
[apikey]: https://developers.google.com/youtube/registering_an_application#create_project
[go]: https://golang.org
