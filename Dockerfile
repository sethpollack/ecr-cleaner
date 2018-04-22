FROM golang:1.9-alpine AS build
RUN apk --no-cache update && \
    apk --no-cache add make ca-certificates git && \
    rm -rf /var/cache/apk/*
WORKDIR /go/src/github.com/sethpollack/ecr-cleaner
RUN go get -u github.com/golang/dep/cmd/dep
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -v -vendor-only
COPY . ./
RUN	CGO_ENABLED=0 GOOS=linux go build -installsuffix cgo -o bin/ecr-cleaner

FROM scratch
LABEL maintainer="Seth Pollack <seth@sethpollack.net>"
COPY --from=build /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=build /go/src/github.com/sethpollack/ecr-cleaner/bin/ecr-cleaner /usr/local/bin/ecr-cleaner
ENTRYPOINT ["/usr/local/bin/ecr-cleaner"]
CMD ["--help"]
