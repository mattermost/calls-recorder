# This dockerfile is used to build Mattermost calls-recorder
# A multi stage build, with golang used as a builder
# and debian sid as runner
ARG GO_VERSION
FROM --platform=$TARGETPLATFORM debian:sid-20240330 as base

# Setup system dependencies
WORKDIR /workdir

ARG TARGETARCH
ARG DEBIAN_FRONTEND=noninteractive

COPY ./build/pkgs_list* .
# hadolint ignore=DL3008,SC2046
RUN set -ex && \
    apt-get update && \
    apt-get install --no-install-recommends -y $(cat pkgs_list_${TARGETARCH}) && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    adduser root pulse-access && \
    mkdir -pv ~/.cache/xdgr

# Create unprivileged user to run the recorder process
RUN groupadd -r calls && useradd -mr -g calls -G audio,video,pulse-access calls

ARG GO_VERSION
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} as builder
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
