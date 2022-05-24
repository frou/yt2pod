FROM golang:alpine AS builder

RUN apk --no-cache add git

RUN mkdir /build_dir
ADD . /build_dir
WORKDIR /build_dir

RUN go build

FROM alpine:latest
RUN apk --no-cache add gcc g++ libc-dev ca-certificates python3 python3-dev py3-pip ffmpeg \
&& pip3 install --disable-pip-version-check yt-dlp \
&& apk del gcc g++ libc-dev python3-dev py3-pip
WORKDIR /root/
COPY --from=builder /build_dir/yt2pod /usr/local/bin/
CMD ["yt2pod", "-dataclean"]
