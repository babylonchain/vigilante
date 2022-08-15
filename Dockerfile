ARG ARCH=amd64

## Image for building
FROM golang:1.18-alpine AS build-env

ARG ARCH
ARG user
ARG pass

RUN apk add --no-cache openssh-client git make build-base linux-headers libc-dev

ENV GO111MODULE=on
ENV GOPRIVATE=github.com/babylonchain/babylon
# Download public key for github.com
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts
# specify github username and password, since vigilante has a private dependency, i.e., babylonchain/babylon
RUN echo "machine github.com login ${user} password ${pass}" >> ~/.netrc

# cache dependency
WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod download

# build vigilante
COPY ./ /app
RUN set -ex \
    && if [ "${ARCH}" = "amd64" ]; then export GOARCH=amd64; fi \
    && if [ "${ARCH}" = "arm32v7" ]; then export GOARCH=arm; fi \
    && if [ "${ARCH}" = "arm64v8" ]; then export GOARCH=arm64; fi \
    && echo "Compiling for $GOARCH" \
    && go build ./cmd/main.go


## Final minimal image with binary only
FROM $ARCH/alpine:3.16 as run

# Install ca-certificates for TLS stuff
RUN apk add --update ca-certificates
WORKDIR /root

# Copy over binaries from the build-env
COPY --from=build-env /app/main /bin/vigilante

# 1317 is used by the Prometheus metrics server
# 8080 is used by the RPC server
EXPOSE 1317 8080

# Run vigilante by default
CMD ["vigilante"]
