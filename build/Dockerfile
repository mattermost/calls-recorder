# This dockerfile is used to build Mattermost calls-recorder
# A multi stage build, with golang used as a builder
# and debian sid as runner
ARG GO_VERSION
FROM --platform=$TARGETPLATFORM debian:sid-20250929 AS base

# Setup system dependencies
WORKDIR /workdir

ARG TARGETARCH
ARG DEBIAN_FRONTEND=noninteractive

COPY ./build/pkgs_list* .
# Prevent QEMU SIGSEGV when cross-building arm64: systemd's postinst script
# runs binaries that crash under QEMU user-mode emulation. We extract systemd
# first with --unpack (skips postinst), replace its postinst with a no-op,
# then run the full install which will configure it harmlessly.
# hadolint ignore=DL3008,SC2046
RUN set -ex && \
    apt-get update && \
    apt-get install --no-install-recommends -y -d $(cat pkgs_list_${TARGETARCH}) && \
    dpkg --force-depends --unpack /var/cache/apt/archives/systemd_*.deb 2>/dev/null || true && \
    printf '#!/bin/sh\nexit 0\n' > /var/lib/dpkg/info/systemd.postinst && \
    apt-get install --no-install-recommends -y -f $(cat pkgs_list_${TARGETARCH}) && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    adduser root pulse-access && \
    mkdir -pv ~/.cache/xdgr

# Create unprivileged user to run the recorder process
RUN groupadd -r calls && useradd -mr -g calls -G audio,video,pulse-access calls

ARG GO_VERSION
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS builder
ARG TARGETOS
ARG TARGETARCH

#GO_BUILD_PLATFORMS holds the platforms that we will build the docker image against
ARG GO_BUILD_PLATFORMS=${TARGETOS}-${TARGETARCH}

# Setup directories structure and compile
COPY . /src
WORKDIR /src
RUN make go-build

FROM base AS runner
ARG TARGETOS
ARG TARGETARCH
COPY --from=builder /src/dist/calls-recorder-${TARGETOS}-${TARGETARCH} /opt/calls-recorder/bin/calls-recorder

# copy binary
COPY ./build/entrypoint.sh .

ENTRYPOINT ["./entrypoint.sh"]
