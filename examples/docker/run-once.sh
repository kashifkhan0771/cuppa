#!/bin/sh
# Post one question with Docker and nothing else installed.
# Usage: SLACK_WEBHOOK=https://hooks.slack.com/... ./run-once.sh
exec docker run --rm -e SLACK_WEBHOOK \
  -v "$PWD/cuppa.yaml:/work/cuppa.yaml:ro" \
  ghcr.io/kashifkhan0771/cuppa:1 post
