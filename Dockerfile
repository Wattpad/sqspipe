FROM alpine:3.4
MAINTAINER Jonathan Harlap <jharlap@users.noreply.github.com>

RUN apk --update add ca-certificates
COPY sqspipe /sqspipe
ENTRYPOINT ["/sqspipe"]
