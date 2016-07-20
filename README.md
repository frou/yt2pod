[yt2pod] is a daemon that monitors YouTube channels and publishes audio
podcasts of them.

A podcast episode needn't be created for every video uploaded to a channel.
They can be filtered based on title and upload date.

A single instance of yt2pod can monitor multiple YouTube channels
simultaneously and publish a separate audio podcast based on each (and even
multiple podcasts based on the same channel).

A built-in webserver serves the following for each podcast.
* RSS Feed: `/meta/{configured_short_name}.xml`
* Artwork: `/meta/{configured_short_name}.jpg`
* Audio Episodes: `/ep/{yt_video_id}.{configured_file_ext}`

---

# Configuration

Main configuration is done using [a JSON file][egcfg].

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

* `short_name` is a unique name that will be used for things like the podcast
feed's file name and logging. For example, for podcast with a `name` of `"This
Week In Bikeshedding"`, use a `short_name` like `"twib"`.

## YouTube Data API

YouTube's Data API is used to query information. If you want to use [your
own][apikey] API key, replace the one in the example config file.

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
  -dataclean
      during initialisation, remove files in the data directory that are irrelevant given the current config
  -syslog
      send log statements to syslog rather than writing them to stderr
  -version
      show version information then exit
```

# Building & Dependencies

With [Go] installed (available in all good package managers):

`$ go get github.com/frou/yt2pod`

The `yt2pod` binary should now be built and located in `$GOPATH/bin`

The only runtime depenency is the [youtube-dl command][ytdl] (available in all
good package managers).

---

# License

```text
The MIT License

Copyright (c) 2015 Duncan Holm

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```


[yt2pod]: https://github.com/frou/yt2pod
[egcfg]: https://github.com/frou/yt2pod/blob/master/example_config.json
[ytdl]: https://rg3.github.io/youtube-dl/
[apikey]: https://developers.google.com/youtube/registering_an_application
[go]: https://golang.org/
