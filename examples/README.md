# Examples

Ready-to-copy setups. Pick the way you want to run cuppa, copy the file, and
change the names. Every config here is loaded by the tests, so they stay in
step with the code.

## Which one do I want?

| I want to... | Use |
|---|---|
| Post from GitHub, with no server | [github-actions/](github-actions/) |
| Post from my own server on a schedule | [systemd/](systemd/), [cron/](cron/), [docker/](docker/) or [kubernetes/](kubernetes/) |
| Post from a Windows computer | [windows/task-scheduler.ps1](windows/task-scheduler.ps1) |
| Keep cuppa running and let it schedule itself | [cuppa.daemon.yaml](cuppa.daemon.yaml) with [systemd/cuppa-daemon.service](systemd/cuppa-daemon.service) or [docker/docker-compose.daemon.yaml](docker/docker-compose.daemon.yaml) |
| Run my own HTTP API | [cuppa.serve.yaml](cuppa.serve.yaml) with [docker/docker-compose.serve.yaml](docker/docker-compose.serve.yaml) or [kubernetes/serve.yaml](kubernetes/serve.yaml) |
| Use Google Apps Script | [appsscript/](appsscript/) |
| Use Power Automate, Zapier, Make or n8n | [power-automate/](power-automate/) |
| Use only the JSON, with curl, Python or Node (no package needed) | [scripts/](scripts/) |
| Use cuppa inside my own Go code | [library/go](library/go) |

## Config files

Start with [cuppa.minimal.yaml](cuppa.minimal.yaml). The full list of options,
with comments, is [cuppa.yaml](cuppa.yaml). Check any config with
`cuppa validate --config <file>`.

| File | Shows |
|---|---|
| [cuppa.minimal.yaml](cuppa.minimal.yaml) | one Slack channel, everything else default |
| [cuppa.yaml](cuppa.yaml) | every option, with comments |
| [cuppa.daemon.yaml](cuppa.daemon.yaml) | a schedule with time zone, weekends and holidays off |
| [cuppa.serve.yaml](cuppa.serve.yaml) | the HTTP API with a token and CORS |
| [cuppa.private-pack.yaml](cuppa.private-pack.yaml) | your own questions ([private-pack/my-pack.yaml](private-pack/my-pack.yaml)) |
| [cuppa.custom-message.yaml](cuppa.custom-message.yaml) | your own message templates |
| [cuppa.fresh-questions.yaml](cuppa.fresh-questions.yaml) | new questions without upgrading (`data.refresh`) |

## One file per chat app

[chat-apps/](chat-apps/): [slack](chat-apps/slack.yaml),
[gchat](chat-apps/gchat.yaml) (Google Chat), [teams](chat-apps/teams.yaml),
[discord](chat-apps/discord.yaml), [mattermost](chat-apps/mattermost.yaml),
[webex](chat-apps/webex.yaml), [webhook](chat-apps/webhook.yaml) (any service
that takes JSON over HTTP) and [many-chats](chat-apps/many-chats.yaml) (several
at once). Each file says how to get the webhook.

## Keep secrets out of files

Webhook URLs and tokens are passwords. Write `${NAME}` in the config and set
the variable in the environment (a GitHub secret, a Docker `.env` file, a
systemd `EnvironmentFile`, a Kubernetes Secret). cuppa warns if it finds a
real URL in the file.

## Versions in these files

The examples use `@v0` (GitHub Action, CDN) and the `:0` Docker tag, which
follow the newest 0.x release. After 1.0 these become `@v1` and `:1`.
