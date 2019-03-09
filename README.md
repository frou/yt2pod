[![Build Status](https://travis-ci.org/frou/yt2pod.svg?branch=master)](https://travis-ci.org/frou/yt2pod)
[![GoDoc](https://godoc.org/github.com/frou/yt2pod?status.svg)](https://godoc.org/github.com/frou/yt2pod)
[![Go Report Card](https://goreportcard.com/badge/github.com/frou/yt2pod)](https://goreportcard.com/report/github.com/frou/yt2pod)

[yt2pod] is a daemon that monitors YouTube channels and publishes audio
podcasts of them.

A podcast episode needn't be created for every video uploaded to a channel.
They can be filtered based on title and upload date.

A single instance of yt2pod can monitor multiple YouTube channels
simultaneously and publish a separate audio podcast based on each (and even
multiple podcasts based on the same channel).

A built-in webserver serves the following for each podcast:

* RSS Feed:
  * `http://YOURDOMAIN.COM/meta/SHORT_NAME.xml`
* Artwork:
  * `http://YOURDOMAIN.COM/meta/SHORT_NAME.jpg`
* Audio Episodes:
  * `http://YOURDOMAIN.COM/ep/id_of_source_youtube_video.EXT`
  * ...
  * ...

The capitalised parts are specified in your config file.

---

# Configuration

Configuration is specified in a JSON file. [Here is an example config file][egcfg].

Each podcast is configured as an element of the `"podcasts"` array. In each:

* `yt_channel` is either the YouTube channel's Username **or** its ID (a
24-character string starting "UC"). Note that on modern YouTube, not every
channel has a Username. Go to the channel's page using your web browser and
look at the URL and you will find either one or the other.

* `epoch` is a date (`"YYYY-MM-DD"` or an empty string to mean the beginning of
time). Videos uploaded before the epoch are ignored.

* `title_filter` is a regular expression. Videos with a title matching it have
a podcast episode created for them. Use an empty string if you want them all.

* `name` is the name of the podcast to be shown to the user in their podcast
client.

* `description` is the description of the podcast to be shown to the user in
their podcast client. If this is omitted or is the empty string, a
matter-of-fact description will be generated.

* `custom_image` is a filesystem path (relative to the data directory - see the
-data flag) for a custom image to use for the podcast's artwork. If this is
omitted or is the empty string, the avatar image of the YouTube user this
podcast is based on will be used.

* `short_name` is a unique name that will be used for things like the podcast
feed's file name and logging. For example, for podcast with a `name` of `"This
Week In Bikeshedding"`, use a `short_name` like `"twib"`.

---

If you do not wish to expose the built-in webserver directly on the internet, you can set a `link_proxy` top-level key in the config file (e.g. `"link_proxy": "https://downloads.obscure-podcasts.com",`). This will cause the download links in the podcast feeds to be prefixed with that URI scheme & host, instead of `http://` and the host yt2pod itself is listening on (which is configured with `serve_host`).

## Command-line Flags

In addition to the config file, there are a handful of command-line flags:

```text
usage:
  yt2pod [flags]

flags:
  -config string
      path to config file (default "config.json")
  -data string
      path to directory to change into and write data (created if needed) (default "data")
  -dataclean
      during initialisation, remove files in the data directory that are irrelevant given the current config
  -syslog
      send log statements to syslog rather than writing them to stderr
  -version
      show version information then exit
```

## YouTube Data API

YouTube's Data API is used to query information. The [example config
file][egcfg] contains an API key that you can use, or you can [get your own API key][apikey] and use it instead.

---

# Building and running using Docker

This project has a [Dockerfile](https://github.com/frou/yt2pod/blob/master/Dockerfile) to facilitate containerized building and deployment.

Get the Dockerfile by cloning this repository and then use the following command to build the Docker image:

`docker build . -t yt2pod`

It can then be run using, for example, the following command:

`docker run -v $PWD:/root -p 8888:8120 yt2pod`

After you see from the output that it has successfully started, visit http://localhost:8888/ in your browser to see what's being served.

---

# Building manually

With the [Go toolchain](https://golang.org/dl/) installed, the following command will download the source code and build it:

`go get github.com/frou/yt2pod`

The `yt2pod` binary should now be built and located in `$GOPATH/bin`

🚨 The `yt2pod` binary calls out to the [youtube-dl][ytdl] command at runtime. You should make sure you have `youtube-dl` installed (it is available in all good package managers).

# Setting up as a Linux service

If you don't want to run using Docker, then you will probably want to set up yt2pod as a service in your Linux distribution of choice.

See hints regarding [systemd](https://github.com/frou/yt2pod/wiki/systemd) and [OpenRC](https://github.com/frou/yt2pod/wiki/OpenRC).

[yt2pod]: https://github.com/frou/yt2pod
[egcfg]: https://github.com/frou/yt2pod/blob/master/config.json
[ytdl]: https://rg3.github.io/youtube-dl/
[apikey]: https://developers.google.com/youtube/registering_an_application
