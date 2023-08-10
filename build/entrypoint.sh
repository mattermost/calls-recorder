#!/usr/bin/env bash
set -euxo pipefail

# Cleanup to be "stateless" on startup, otherwise pulseaudio daemon can't start
rm -rf /var/run/pulse /var/lib/pulse /root/.config/pulse

# Start pulseaudio as system wide daemon; for debugging it helps to start in non-daemon mode
pulseaudio -D --verbose --exit-idle-time=-1 --system --disallow-exit --disable-shm=true --log-time=true

# Load audio sink
pactl load-module module-null-sink sink_name="grab" sink_properties=device.description="monitorOUT"

# Forward signals to service
RECORDER_PID=0
RECORDER_PID_FILE=/tmp/recorder.pid
RECORDER_EXIT_CODE_FILE=/tmp/recorder.ecode

exit_handler() {
  # If exit code file doesn't exit we exit with failure.
  if [ ! -f "$RECORDER_EXIT_CODE_FILE" ]; then
    echo "exit code not found"
    exit 1
  fi

  EXIT_CODE=`cat $RECORDER_EXIT_CODE_FILE`

  exit $EXIT_CODE
}

# SIGTERM handler
term_handler() {
  # We unsubscribe the handler to avoid issues if receiving another
  # SIGTERM while in here (e.g. stopping twice).
  trap - SIGTERM

  # If pid file doesn't exit we exit with failure
  # as the recording process hasn't started properly.
  if [ ! -f "$RECORDER_PID_FILE" ]; then
    echo "pid not found"
    exit 1
  fi

  RECORDER_PID=`cat $RECORDER_PID_FILE`

  # Relaying the SIGTERM to the recorder's process.
  kill -SIGTERM "$RECORDER_PID"
  # Wait for the recorder's process to exit.
  tail --pid="$RECORDER_PID" -f /dev/null

  # Wait a second for the recorder's exit code to be saved.
  sleep 1

  exit_handler
}

# On callback, kill the last background process, which is `tail -f /dev/null` and execute the specified handler
trap 'kill ${!}; term_handler' SIGTERM

RECORDER_USER=calls

# Give permission to write recording files.
chown -R $RECORDER_USER:$RECORDER_USER /recs

# Turn off trace flag so that we avoid logging all the env variables.
set +x

# Run service as unprivileged user.
runuser -l $RECORDER_USER -c \
  "SITE_URL=$SITE_URL \
  AUTH_TOKEN=$AUTH_TOKEN \
  CALL_ID=$CALL_ID \
  THREAD_ID=$THREAD_ID \
  WIDTH=${WIDTH:-0} \
  HEIGHT=${HEIGHT:-0} \
  VIDEO_RATE=${VIDEO_RATE:-0} \
  AUDIO_RATE=${AUDIO_RATE:-0} \
  FRAME_RATE=${FRAME_RATE:-0} \
  VIDEO_PRESET=${VIDEO_PRESET:-} \
  OUTPUT_FORMAT=${OUTPUT_FORMAT:-} \
  DEV_MODE=${DEV_MODE:-false} \
  XDG_RUNTIME_DIR=/home/$RECORDER_USER/.cache/xdgr \
  /bin/bash -c '/opt/calls-recorder/bin/calls-recorder; echo \$? > ${RECORDER_EXIT_CODE_FILE}'" &

# Turn track flag back on
set -x

# Wait forever
wait ${!}

exit_handler
