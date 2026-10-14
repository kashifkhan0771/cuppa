#!/bin/sh
# Post a question to a Slack (or Google Chat) webhook with curl and jq only.
# Usage: SLACK_WEBHOOK=https://hooks.slack.com/services/... ./post-to-slack.sh
set -e
URL=https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json

text=$(curl -fsS "$URL" | jq -r '[.questions[] | select(.sfw == "safe")] | (. as $q | $q[now | floor % ($q | length)]) | .text')
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d "$(jq -n --arg text "☕ *Today's watercooler:* $text" '{text: $text}')" \
  "$SLACK_WEBHOOK"
