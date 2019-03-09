FROM golang:alpine

RUN mkdir -p /go/src/github.com/frou/yt2pod \
 && apk add --no-cache git
ADD . /go/src/github.com/frou/yt2pod/

WORKDIR /go/src/github.com/frou/yt2pod/
RUN go get -d ./... \
 && go install

FROM alpine:latest
RUN apk --no-cache add ca-certificates python3 py3-pip ffmpeg \
&& pip3 install --disable-pip-version-check youtube-dl \
&& apk del py3-pip
WORKDIR /root/
COPY --from=0 /go/bin/yt2pod /usr/local/bin/
CMD ["yt2pod", "-dataclean"]
