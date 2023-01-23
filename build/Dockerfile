# This dockerfile is used to build Mattermost calls-recorder
# A multi stage build, with golang used as a builder
# and ubuntu:22.04 as runner
ARG GO_IMAGE=golang:1.18@sha256:fa71e1447cb0241324162a6c51297206928d755b16142eceec7b809af55061e5
ARG PLATFORM
ARG CHROMEDRIVER
ARG CHROME
ARG CODECS

FROM ubuntu:22.04@sha256:817cfe4672284dcbfee885b1a66094fd907630d610cab329114d036716be49ba as base-amd64

FROM arm64v8/ubuntu:jammy@sha256:e77aa65a8a2bccbc47b96b4256995dd7ff447024ed5319527040f7cc465f6511 as base-arm64

ARG PLATFORM

FROM base-${PLATFORM:?} as final

# Setup system dependencies
WORKDIR /workdir

# Workaround for Ubuntu 22.04 `apt update` failing when running under Docker < 20.10.9
# https://stackoverflow.com/questions/71941032/why-i-cannot-run-apt-update-inside-a-fresh-ubuntu22-04
RUN sed -i -e 's/^APT/# APT/' -e 's/^DPkg/# DPkg/' /etc/apt/apt.conf.d/docker-clean

ARG CHROMEDRIVER
ARG CHROME
ARG CODECS
ARG DEBIAN_FRONTEND=noninteractive

RUN echo "PLATFORM is ${PLATFORM:?}"
RUN echo "CHROMEDRIVER is ${CHROMEDRIVER:?}"
RUN echo "CHROME is ${CHROME:?}"
RUN echo "CODECS is ${CODECS:?}"

COPY ./build/pkgs_list .
# hadolint ignore=DL3008,SC2046
RUN set -ex && \
    apt-get update && \
    apt-get install --no-install-recommends -y $(cat pkgs_list) && \
    apt-get update && \
    wget --progress=dot:giga -N \
      -O chromium-chromedriver.deb \
      ${CHROMEDRIVER:?}  && \
    wget --progress=dot:giga -N \
      -O chromium-codecs-ffmpeg-extra.deb \
      ${CODECS:?} && \
    wget --progress=dot:giga -N \
      -O chromium-browser.deb \
      ${CHROME:?} && \
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
ARG PLATFORM

#GO_BUILD_PLATFORMS holds the platforms that we will build the docker image against
ARG GO_BUILD_PLATFORMS=linux-${PLATFORM:?}

# Setup directories structure and compile
COPY . /src
WORKDIR /src
RUN make go-build

FROM final AS runner
ARG PLATFORM
COPY --from=builder /src/dist/calls-recorder-linux-${PLATFORM:?} /opt/calls-recorder/bin/calls-recorder

# copy binary
COPY ./build/entrypoint.sh .

ENTRYPOINT ["./entrypoint.sh"]
