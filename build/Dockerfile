# This dockerfile is used to build Mattermost calls-recorder
# A multi stage build, with golang used as a builder
# and ubuntu:22.04 as runner
ARG GO_IMAGE=golang:1.18@sha256:fa71e1447cb0241324162a6c51297206928d755b16142eceec7b809af55061e5
ARG ARCH
ARG RUNNER_IMAGE

FROM ${RUNNER_IMAGE:?} as base

# Setup system dependencies
WORKDIR /workdir

ARG DEBIAN_FRONTEND=noninteractive

COPY ./build/pkgs_list .
# hadolint ignore=DL3008,SC2046
RUN set -ex && \
    apt-get update && \
    apt-get install --no-install-recommends -y $(cat pkgs_list) && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    adduser root pulse-access && \
    mkdir -pv ~/.cache/xdgr

# Create unprivileged user to run the recorder process
RUN groupadd -r calls && useradd -mr -g calls -G audio,video,pulse-access calls

# hadolint ignore=DL3006
FROM ${GO_IMAGE} as builder
ARG ARCH

#GO_BUILD_PLATFORMS holds the platforms that we will build the docker image against
ARG GO_BUILD_PLATFORMS=linux-${ARCH:?}

# Setup directories structure and compile
COPY . /src
WORKDIR /src
RUN make go-build

FROM base AS runner
ARG ARCH
COPY --from=builder /src/dist/calls-recorder-linux-${ARCH:?} /opt/calls-recorder/bin/calls-recorder

# copy binary
COPY ./build/entrypoint.sh .

ENTRYPOINT ["./entrypoint.sh"]
