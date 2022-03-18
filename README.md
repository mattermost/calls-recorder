# calls-recorder

A headless recorder for [Mattermost Calls](https://github.com/mattermost/mattermost-plugin-calls)

## Usage

```sh
make docker
```

```sh
SITE_URL="http://localhost:8065" USERNAME="calls-recorder" PASSWORD="" TEAM_NAME="calls" CHANNEL_ID="he1kbdi6kjb3fpte7og9z1zsyo" make run
```

```sh
make stop
```

## Acknowledgements

Kudos to https://github.com/livekit/livekit-recorder for the idea and code that's been ported here.
