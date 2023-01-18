#!/usr/bin/env bash
set -euxo pipefail

# Cleanup to be "stateless" on startup, otherwise pulseaudio daemon can't start
rm -rf /var/run/pulse /var/lib/pulse /root/.config/pulse

# Start pulseaudio as system wide daemon; for debugging it helps to start in non-daemon mode
pulseaudio -D --verbose --exit-idle-time=-1 --system --disallow-exit

# Load audio sink
pactl load-module module-null-sink sink_name="grab" sink_properties=device.description="monitorOUT"

# Forward signals to service
RECORDER_PID=0
RECORDER_PID_FILE=/tmp/recorder.pid

# SIGTERM handler
term_handler() {
  RECORDER_PID=`cat $RECORDER_PID_FILE`
  if [ $RECORDER_PID -ne 0 ]; then
    kill -SIGTERM "$RECORDER_PID"
    tail --pid="$RECORDER_PID" -f /dev/null
  fi
  exit 143; # 128 + 15 -- SIGTERM
}

# On callback, kill the last background process, which is `tail -f /dev/null` and execute the specified handler
trap 'kill ${!}; term_handler' SIGTERM

# DEV_MODE is optional so we need to check whether it's set before relaying it to the recorder app.
DEV="${DEV_MODE:-false}"

RECORDER_USER=calls

# Give permission to write recording files.
chown -R $RECORDER_USER:$RECORDER_USER /recs

# Run service as unprivileged user.
runuser -l $RECORDER_USER -c \
  "SITE_URL=$SITE_URL \
  AUTH_TOKEN=$AUTH_TOKEN \
  CALL_ID=$CALL_ID \
  THREAD_ID=$THREAD_ID \
  DEV_MODE=$DEV \
  XDG_RUNTIME_DIR=/home/$RECORDER_USER/.cache/xdgr \
  /opt/calls-recorder/bin/calls-recorder" &

# Wait forever
wait ${!}
