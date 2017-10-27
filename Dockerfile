#######################################################################
# Dockerfile to install github.com/snakelayer/discord-oversessions
#
# run "docker build . -t snakelayer/discord-oversessions"
#######################################################################

FROM golang:1.9.2

RUN go get github.com/snakelayer/discord-oversessions

RUN go install github.com/snakelayer/discord-oversessions

RUN rm -rf /go/src

VOLUME /BattleTags

STOPSIGNAL SIGINT

ENTRYPOINT ["discord-oversessions", "-battleTags", "/BattleTags/battletags"]
