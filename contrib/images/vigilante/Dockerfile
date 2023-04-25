## Image for building
FROM golang:1.20-alpine AS build-env


# TARGETPLATFORM should be one of linux/amd64 or linux/arm64.
ARG TARGETPLATFORM="linux/amd64"
# Version to build. Default is the Git HEAD.
ARG VERSION="HEAD"

# Use muslc for static libs
ARG BUILD_TAGS="muslc"

RUN apk add --no-cache --update openssh git make build-base linux-headers libc-dev \
                                pkgconfig zeromq-dev musl-dev alpine-sdk libsodium-dev \
                                libzmq-static libsodium-static gcc

# Build
WORKDIR /go/src/github.com/babylonchain/vigilante
# Cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonchain/vigilante/
RUN go mod download
# Copy the rest of the files
COPY ./ /go/src/github.com/babylonchain/vigilante/
RUN git checkout ${VERSION}

# Cosmwasm - Download correct libwasmvm version
RUN WASMVM_VERSION=$(go list -m github.com/CosmWasm/wasmvm | cut -d ' ' -f 2) && \
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm_muslc.$(uname -m).a \
        -O /lib/libwasmvm_muslc.a && \
    # verify checksum
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/checksums.txt -O /tmp/checksums.txt && \
    sha256sum /lib/libwasmvm_muslc.a | grep $(cat /tmp/checksums.txt | grep $(uname -m) | cut -d ' ' -f 1)

RUN CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
    CGO_ENABLED=1 \
    BUILD_TAGS=$BUILD_TAGS \
    LINK_STATICALLY=true \
    make build

FROM alpine:3.16 AS run
# Create a user
RUN addgroup --gid 1138 -S vigilante && adduser --uid 1138 -S vigilante -G vigilante
RUN apk add bash curl jq

# Label should match your github repo
LABEL org.opencontainers.image.source="https://github.com/babylonchain/vigilante:${VERSION}"

COPY --from=build-env /go/src/github.com/babylonchain/vigilante/build/vigilante /bin/vigilante

# Set home directory and user
WORKDIR /home/vigilante
RUN chown -R vigilante /home/vigilante
USER vigilante
