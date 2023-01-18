# This dockerfile is used to build Mattermost calls-recorder
# A multi stage build, with golang used as a builder
# and ubuntu:22.04 as runner
ARG GO_IMAGE=golang:1.18@sha256:fa71e1447cb0241324162a6c51297206928d755b16142eceec7b809af55061e5

FROM ubuntu:22.04@sha256:817cfe4672284dcbfee885b1a66094fd907630d610cab329114d036716be49ba as base

# Setup system dependencies
WORKDIR /workdir

# Workaround for Ubuntu 22.04 `apt update` failing when running under Docker < 20.10.9
# https://stackoverflow.com/questions/71941032/why-i-cannot-run-apt-update-inside-a-fresh-ubuntu22-04
RUN sed -i -e 's/^APT/# APT/' -e 's/^DPkg/# DPkg/' /etc/apt/apt.conf.d/docker-clean

ARG CHROMIUM_VERSION=109.0.5414.74-0
ARG DEBIAN_FRONTEND=noninteractive
COPY ./build/pkgs_list .
# hadolint ignore=DL3008,SC2046
RUN set -ex && \
    apt-get update && \
    apt-get install --no-install-recommends -y $(cat pkgs_list) && \
    apt-get update && \
    wget --progress=dot:giga -N \
      -O chromium-chromedriver.deb \
      https://launchpad.net/~savoury1/+archive/ubuntu/chromium/+files/chromium-chromedriver_${CHROMIUM_VERSION}ubuntu0.22.04.1sav1_amd64.deb  && \
    wget --progress=dot:giga -N \
      -O chromium-codecs-ffmpeg-extra.deb \
      https://launchpad.net/~savoury1/+archive/ubuntu/chromium/+files/chromium-codecs-ffmpeg-extra_${CHROMIUM_VERSION}ubuntu0.22.04.1sav1_amd64.deb && \
    wget --progress=dot:giga -N \
      -O chromium-browser.deb \
      https://launchpad.net/~savoury1/+archive/ubuntu/chromium/+files/chromium-browser_${CHROMIUM_VERSION}ubuntu0.22.04.1sav1_amd64.deb && \
    apt-get install --no-install-recommends -y ./chromium-chromedriver.deb \
    ./chromium-codecs-ffmpeg-extra.deb \
    ./chromium-browser.deb && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    adduser root pulse-access && \
    mkdir -pv ~/.cache/xdgr

# Create unprivileged user to run the recorder process
RUN groupadd -r calls && useradd -mr -g calls -G audio,video,pulse-access calls

# hadolint ignore=DL3006
FROM ${GO_IMAGE} as builder

#GO_BUILD_PLATFORMS holds the platforms that we will build the docker image against
ARG GO_BUILD_PLATFORMS=linux-amd64

# Setup directories structure and compile
COPY . /src
WORKDIR /src
RUN make go-build

FROM base AS runner
COPY --from=builder /src/dist/calls-recorder-linux-amd64 /opt/calls-recorder/bin/calls-recorder

# copy binary
COPY ./build/entrypoint.sh .

ENTRYPOINT ["./entrypoint.sh"]
