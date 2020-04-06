FROM alpine:3.4
MAINTAINER Timothy Lock <timothy.lock@wattpad.com>

RUN apk --update add ca-certificates
COPY sqspipe /sqspipe
ENTRYPOINT ["/sqspipe"]
