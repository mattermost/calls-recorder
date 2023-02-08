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
RECORDER_EXIT_CODE=143 # 128 + 15 -- SIGTERM
RECORDER_EXIT_CODE_FILE=/tmp/recorder.ecode

# SIGTERM handler
term_handler() {
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

  # If exit code file doesn't exit we exit with failure.
  if [ ! -f "$RECORDER_EXIT_CODE_FILE" ]; then
    echo "exit code not found"
    exit 1
  fi

  EXIT_CODE=`cat $RECORDER_EXIT_CODE_FILE`

  # Sum the recorder's exit code to the base exit code to
  # inform the job handler in case of failure.
  RECORDER_EXIT_CODE=$(($RECORDER_EXIT_CODE+$EXIT_CODE))

  exit $RECORDER_EXIT_CODE;
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
  /bin/bash -c '/opt/calls-recorder/bin/calls-recorder; echo \$? > ${RECORDER_EXIT_CODE_FILE}'" &

# Wait forever
wait ${!}
