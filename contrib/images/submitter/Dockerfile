## Image for building
FROM golang:1.18-alpine AS build-env

ARG user
ARG pass

RUN apk add --no-cache openssh-client git make build-base linux-headers libc-dev \
                       pkgconfig zeromq-dev musl-dev alpine-sdk libsodium-dev \
                       libzmq-static libsodium-static

ENV GO111MODULE=on
ENV GOPRIVATE=github.com/babylonchain/babylon
# Download public key for github.com
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts
# specify github username and password, since vigilante has a private dependency, i.e., babylonchain/babylon
RUN echo "machine github.com login ${user} password ${pass}" >> ~/.netrc

# cache dependency
WORKDIR /work
COPY go.mod go.sum /work/
RUN go mod download

# build vigilante
COPY ./ /work
RUN CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
    CGO_ENABLED=1 \
    GOOS=linux \
    go build -v --ldflags '-extldflags "-static" -v' ./cmd/main.go


## Final minimal image with binary only
FROM alpine:3.16 as run

# Install ca-certificates for TLS stuff
RUN apk add --update ca-certificates
COPY contrib/images/submitter/wrapper.sh /usr/bin/wrapper.sh

# Copy over binaries from the build-env
VOLUME /app
COPY --from=build-env /work/main /app/vigilante
WORKDIR /app

# 1317 is used by the Prometheus metrics server
# 8080 is used by the RPC server
EXPOSE 1317 8080

ENTRYPOINT ["/usr/bin/wrapper.sh"]
CMD []
STOPSIGNAL SIGTERM
