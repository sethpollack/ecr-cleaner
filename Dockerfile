FROM golang:1.21-alpine AS builder
WORKDIR /
RUN apk --no-cache add ca-certificates git && rm -rf /var/cache/apk/*
ENV GO111MODULE=auto
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ecr-cleaner

FROM alpine:3
RUN apk --no-cache add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /usr/local/bin/
RUN adduser -D -H -s /usr/sbin/nologin app
USER app
# RUN mkdir /home/app
COPY --from=builder ecr-cleaner ./
COPY aws/ /home/app/.aws/
ENTRYPOINT ["/usr/local/bin/ecr-cleaner"]
CMD ["--help"]
