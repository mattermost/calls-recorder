# `calls-recorder`

A headless recorder for [Mattermost Calls](https://github.com/mattermost/mattermost-plugin-calls).
This works by running a Docker container that does the following:

- Spawns a Chromium browser running on a [`Xvfb`](https://www.x.org/releases/X11R7.6/doc/man/man1/Xvfb.1.xhtml) display.
- Logs a bot user and connects it to the call using [`chromedp`](https://github.com/chromedp/chromedp).
- Grabs the screen and audio using [`ffmpeg`](https://ffmpeg.org) and outputs to a file.
- Creates a post in the specified channel with the recording file attached.

## Usage

This program is **not** meant to be run directly (nor manually) other than for development/testing purposes. In fact, this is automatically used by the [`calls-offloader`](https://github.com/mattermost/calls-offloader) service to run recording jobs. Please refer to that project if you are looking to enable call recordings in your Mattermost instance.

## Manual execution (testing only)

### Fetch the latest image

The latest official docker image can be found at https://hub.docker.com/r/mattermost/calls-recorder.

```
docker pull mattermost/calls-recorder:latest
```

### Run the container

```
docker run --network=host --name calls-recorder -e "SITE_URL=http://127.0.0.1:8065/" -e "AUTH_TOKEN=ohqd1phqtt8m3gsfg8j5ymymqy" -e "CALL_ID=9c86b3q57fgfpqr8jq3b9yjweh" -e "POST_ID=e4pdmi6rqpn7pp9sity9hiza3r" -e "DEV_MODE=true" -v calls-recorder-volume:/data mattermost/calls-recorder
```

> **_Note_** 
>
> This process requires Mattermost Server >= v7.6 with the latest Mattermost Calls version installed.

> **_Note_**
> - `SITE_URL`: The URL pointing to the Mattermost installation.
> - `AUTH_TOKEN`: The authentication token for the Calls bot.
> - `CALL_ID`: The channel ID in which the call to record has been started.
> - `POST_ID`: The post ID the recording file should be attached to.

> **_Note_**
>
> The auth token for the bot can be found through this SQL query:
> ```sql
> SELECT Token FROM Sessions JOIN Bots ON Sessions.UserId = Bots.UserId AND Bots.OwnerId = 'com.mattermost.calls' ORDER BY Sessions.CreateAt DESC LIMIT 1;
> ```

### Development

Run `make help` to see available options.

## Acknowledgements

Kudos to https://github.com/livekit/livekit-recorder for the idea and code that's been ported here.
