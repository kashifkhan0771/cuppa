# Google Apps Script

Two scripts, depending on whether you run cuppa yourself.

| File | Needs | What it does |
|---|---|---|
| [daily-question.gs](daily-question.gs) | only a chat webhook | Reads the public `questions.json`, picks the question of the day and posts it to Google Chat or Slack. |
| [from-cuppa-api.gs](from-cuppa-api.gs) | your own `cuppa serve` on a public HTTPS address | Calls the API: read today's question, ask the server to post, or use it in a Google Sheet cell. |

Start with `daily-question.gs`. The setup steps are in the comment at the top
of the file. In short: paste it into a new project at
[script.google.com](https://script.google.com), add the `WEBHOOK_URL` script
property, run `logDailyQuestion` to try it, then add a daily time trigger.

The picker is the same as cuppa's `daily` mode, so a team using this script
and a team using the CLI with the same `seed` see the same question each day.
