#!/bin/sh
# Use the public questions.json with nothing but curl and jq.
# No cuppa install, no account. Needs: curl, jq.
URL=https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json

# a random work-safe question
curl -fsS "$URL" | jq -r '[.questions[] | select(.sfw == "safe")] | (. as $q | $q[now | floor % ($q | length)]) | .text'

# every category and how many questions it has
curl -fsS "$URL" | jq -r '.categories[] | "\(.id)\t\(.count)"'

# all questions with a tag
curl -fsS "$URL" | jq -r '.questions[] | select(.tags // [] | index("work")) | "\(.id)\t\(.text)"'
