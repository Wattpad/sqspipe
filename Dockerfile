FROM alpine:3.4
MAINTAINER Wattpad <engineers@wattpad.com>

RUN apk --update add ca-certificates
COPY sqspipe /sqspipe
ENTRYPOINT ["/sqspipe"]
