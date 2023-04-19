## Image for building
FROM golang:1.19-alpine AS build-env

RUN apk add --no-cache --update openssh git make build-base linux-headers libc-dev \
                       pkgconfig zeromq-dev musl-dev alpine-sdk libsodium-dev \
                       libzmq-static libsodium-static

# cache dependency
WORKDIR /work
COPY go.mod go.sum /work/
RUN go mod download

# build vigilante
COPY ./ /work

# Cosmwasm - Download correct libwasmvm version
RUN WASMVM_VERSION=$(go list -m github.com/CosmWasm/wasmvm | cut -d ' ' -f 2) && \
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm_muslc.$(uname -m).a \
        -O /lib/libwasmvm_muslc.a && \
    # verify checksum
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/checksums.txt -O /tmp/checksums.txt && \
    sha256sum /lib/libwasmvm_muslc.a | grep $(cat /tmp/checksums.txt | grep $(uname -m) | cut -d ' ' -f 1)

RUN CGO_LDFLAGS="-lstdc++ -lm -lsodium" \
    BUILD_TAGS="muslc" \
    LINK_STATICALLY=true \
    make build-linux

## Final minimal image with binary only
FROM alpine:3.16 as run

# Install ca-certificates for TLS stuff
RUN apk add --update ca-certificates
COPY contrib/images/submitter/wrapper.sh /usr/bin/wrapper.sh

# Copy over binaries from the build-env
VOLUME /app
COPY --from=build-env /work/build/vigilante /app/vigilante
WORKDIR /app

# 1317 is used by the Prometheus metrics server
# 8080 is used by the RPC server
EXPOSE 1317 8080

ENTRYPOINT ["/usr/bin/wrapper.sh"]
CMD []
STOPSIGNAL SIGTERM
