FROM ubuntu:latest

WORKDIR /workdir

ARG DEBIAN_FRONTEND=noninteractive
RUN set -ex && apt-get update && apt-get install -y ffmpeg pulseaudio xvfb wget unzip fonts-emojione

# install chrome
RUN wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && \
    apt-get update && apt-get install -y ./google-chrome-stable_current_amd64.deb && \
    rm google-chrome-stable_current_amd64.deb

# install chromedriver
RUN wget -N http://chromedriver.storage.googleapis.com/2.46/chromedriver_linux64.zip && \
    unzip chromedriver_linux64.zip && \
    chmod +x chromedriver && \
    mv -f chromedriver /usr/local/bin/chromedriver && \
    rm chromedriver_linux64.zip

# clean
RUN apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# add root user to group for pulseaudio access
RUN adduser root pulse-access

# create xdg_runtime_dir
RUN mkdir -pv ~/.cache/xdgr

# copy binary
COPY entrypoint.sh .
COPY main .

ENTRYPOINT ["./entrypoint.sh"]
