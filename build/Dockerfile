# This dockerfile is used to build Mattermost calls-recorder
# A multi stage build, with golang used as a builder
# and ubuntu:22.04 as runner
ARG GO_IMAGE=golang:1.18@sha256:fa71e1447cb0241324162a6c51297206928d755b16142eceec7b809af55061e5
# hadolint ignore=DL3006
FROM ${GO_IMAGE} as builder

#GO_BUILD_PLATFORMS holds the platforms that we will build the docker image against
ARG GO_BUILD_PLATFORMS=linux-amd64

# Setup directories structure and compile
COPY . /src
WORKDIR /src
RUN make go-build

FROM ubuntu:22.04@sha256:817cfe4672284dcbfee885b1a66094fd907630d610cab329114d036716be49ba as runner
COPY --from=builder /src/dist/calls-recorder-linux-amd64 /opt/calls-recorder/bin/calls-recorder

# Setup system dependencies
WORKDIR /workdir

# Workaround for Ubuntu 22.04 `apt update` failing when running under Docker < 20.10.9
# https://stackoverflow.com/questions/71941032/why-i-cannot-run-apt-update-inside-a-fresh-ubuntu22-04
RUN sed -i -e 's/^APT/# APT/' -e 's/^DPkg/# DPkg/' /etc/apt/apt.conf.d/docker-clean

ARG DEBIAN_FRONTEND=noninteractive
RUN set -ex && \
    apt-get update && \
    apt-get install --no-install-recommends -y ffmpeg=7:4.4.2-0ubuntu0.22.04.1 pulseaudio=1:15.99.1+dfsg1-1ubuntu2 \
      xvfb=2:21.1.3-2ubuntu2.3 wget=1.21.2-2ubuntu1 unzip=6.0-26ubuntu3.1 fonts-emojione=2.2.6+16.10.20160804-0ubuntu2 ca-certificates=20211016 && \
    apt-get update && \
    wget --progress=dot:giga -N https://launchpad.net/~savoury1/+archive/ubuntu/chromium/+files/chromium-chromedriver_107.0.5304.121-0ubuntu0.22.04.1sav0_amd64.deb && \
    wget --progress=dot:giga -N https://launchpad.net/~savoury1/+archive/ubuntu/chromium/+files/chromium-codecs-ffmpeg-extra_107.0.5304.121-0ubuntu0.22.04.1sav0_amd64.deb && \
    wget --progress=dot:giga -N https://launchpad.net/~savoury1/+archive/ubuntu/chromium/+files/chromium-browser_107.0.5304.121-0ubuntu0.22.04.1sav0_amd64.deb && \
    apt-get install --no-install-recommends -y ./chromium-chromedriver_107.0.5304.121-0ubuntu0.22.04.1sav0_amd64.deb \
    ./chromium-codecs-ffmpeg-extra_107.0.5304.121-0ubuntu0.22.04.1sav0_amd64.deb \
    ./chromium-browser_107.0.5304.121-0ubuntu0.22.04.1sav0_amd64.deb && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    adduser root pulse-access && \
    mkdir -pv ~/.cache/xdgr

# copy binary
COPY ./build/entrypoint.sh .

ENTRYPOINT ["./entrypoint.sh"]
