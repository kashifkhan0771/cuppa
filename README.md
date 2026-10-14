# cuppa

*Grab a cuppa. Answer today's question.*

cuppa posts a watercooler question to your team chat: Slack, Google Chat,
Microsoft Teams, Discord, Mattermost, Webex, or anything with a webhook. There is no central server and nothing to sign
up for. The questions are plain data in this repo, and you run cuppa
wherever you like: GitHub Actions, cron, Docker, or your laptop.

> **Status:** early (0.x). Config and commands may still change before 1.0.

## Quick start: Slack with GitHub Actions (2 minutes)

1. In Slack, create an [incoming webhook](https://api.slack.com/messaging/webhooks) for your channel.
2. In any GitHub repo, add it as a secret named `SLACK_WEBHOOK`
   (Settings > Secrets and variables > Actions).
3. Add `.github/workflows/watercooler.yml`:

```yaml
name: Watercooler
on:
  schedule:
    - cron: "0 10 * * 1-5" # weekdays, 10:00 UTC
  workflow_dispatch:
permissions:
  contents: read
jobs:
  post:
    runs-on: ubuntu-latest
    steps:
      - uses: kashifkhan0771/cuppa@v0
        with:
          slack_webhook: ${{ secrets.SLACK_WEBHOOK }}
```

Run it once from the Actions tab to see it work. Other chat apps work the
same way with their own input (see [Chat apps](#chat-apps)); any other
service can use `webhook_url`.

Action inputs: `config`, `slack_webhook`, `gchat_webhook`, `teams_webhook`,
`discord_webhook`, `mattermost_webhook`, `webex_token` with `webex_room_id`,
`webhook_url`, `mode` (`daily`, `random`, `no-repeat`; default `daily`), `seed` (default:
the repo name), `categories` (comma separated), `sfw` (`safe` or `casual`),
`dry_run`. Outputs: `question_id`, `question_text`.
See [action.yml](action.yml) for details.

## Install the command line tool

- **Download:** binaries for Linux, macOS and Windows are on the
  [releases page](https://github.com/kashifkhan0771/cuppa/releases),
  with a `checksums.txt` to verify them.
- **Go:** `go install github.com/kashifkhan0771/cuppa/cmd/cuppa@latest`
- **Docker:** `docker run --rm ghcr.io/kashifkhan0771/cuppa next`

## Use it

```sh
cuppa init                 # answer 3 questions, get a cuppa.yaml
export SLACK_WEBHOOK='https://hooks.slack.com/services/...'
cuppa post --dry-run       # see what would be posted
cuppa post                 # post it
```

Other commands:

| Command | What it does |
|---|---|
| `cuppa next` | print a question without posting (`--json`, `--category`, `--tag`, `--mode`) |
| `cuppa post` | post to every target in the config (`--target`, `--dry-run`, `--json`) |
| `cuppa daemon` | keep running and post on `schedule.cron` (`--dry-run`) |
| `cuppa list` / `cuppa categories` | browse the questions |
| `cuppa validate [file]` | check a config file or a question pack |
| `cuppa history show\|reset` | the no-repeat history |
| `cuppa version` | version of cuppa and of the questions |

Exit codes: `0` ok, `1` error, `2` config or usage error, `3` posting failed
on at least one target, `4` no question matches the filter.

With Docker, mount your config into `/work`:

```sh
docker run --rm -e SLACK_WEBHOOK -v "$PWD/cuppa.yaml:/work/cuppa.yaml:ro" \
  ghcr.io/kashifkhan0771/cuppa post
```

## Chat apps

| App | Config `type` | What you need |
|---|---|---|
| Slack | `slack` | an [incoming webhook](https://api.slack.com/messaging/webhooks) URL |
| Google Chat | `gchat` | a space webhook: space name > Apps & integrations > Webhooks |
| Microsoft Teams | `teams` | a Workflows webhook: channel > Workflows > "Send webhook alerts to a channel" |
| Discord | `discord` | a channel webhook: channel settings > Integrations > Webhooks |
| Mattermost | `mattermost` | an incoming webhook: Integrations > Incoming Webhooks |
| Webex | `webex` | a bot token from [developer.webex.com](https://developer.webex.com/my-apps), the bot added to the space, and the space's room ID |
| Anything else | `webhook` | a URL; you choose the method, headers and JSON body |

Teams uses Workflows because Microsoft is retiring the older Office 365
connector webhooks. Discord posts never ping `@everyone` or anyone else.

## Run as a daemon

Instead of cron or GitHub Actions, cuppa can keep running and post on its
own schedule. Add this to `cuppa.yaml`:

```yaml
schedule:
  cron: "0 10 * * 1-5"      # 10:00, Monday to Friday
  timezone: Asia/Karachi    # empty means the computer's own
  skip_dates: ["2026-12-25"]
```

Then run `cuppa daemon`. `cuppa validate` shows when the next post will be.
The cron line takes numbers only (no `MON` or `JAN`). If the computer was
asleep or off at post time and wakes up more than an hour late, that post
is skipped rather than sent late. A failed post is logged and the daemon
keeps going. It stops cleanly on Ctrl+C or SIGTERM.

With Docker (the `cuppa-home` volume keeps the no-repeat history):

```sh
docker run -d --restart unless-stopped -e SLACK_WEBHOOK \
  -v "$PWD/cuppa.yaml:/work/cuppa.yaml:ro" -v cuppa-home:/home/nonroot \
  ghcr.io/kashifkhan0771/cuppa daemon
```

As a systemd service (`/etc/systemd/system/cuppa.service`):

```ini
[Unit]
Description=cuppa watercooler questions
After=network-online.target

[Service]
ExecStart=/usr/local/bin/cuppa daemon --config /etc/cuppa/cuppa.yaml
EnvironmentFile=/etc/cuppa/env
DynamicUser=yes
StateDirectory=cuppa
Environment=XDG_STATE_HOME=/var/lib
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Config

`cuppa init` writes a starter file. The full set of options, with comments,
is in [examples/cuppa.yaml](examples/cuppa.yaml). The main ideas:

- **Modes:** `daily` gives everyone with the same `seed` the same question
  each day, with no saved state. `random` picks any question. `no-repeat`
  never repeats until every question was used, and keeps a small history file.
- **Secrets** go in environment variables: write `webhook: ${SLACK_WEBHOOK}`.
  cuppa warns if a webhook URL or token is pasted into the file as plain text.
- **Message:** `message.template` is a Go template. `{{ .Bold "text" }}`
  writes bold the right way for each chat app.
- **Your own questions:** list private question files under
  `data.extra_questions`. They use the same format as the files in
  [questions/](questions/). Check them with `cuppa validate my-pack.yaml`.

cuppa looks for its config in `./cuppa.yaml`, then your user config folder,
then `~/.config/cuppa/cuppa.yaml`, or wherever `--config` points.

## Just the questions (no install)

Every release publishes the questions as JSON, so scripts, spreadsheets,
Google Apps Script or Power Automate can use them directly:

- Latest 0.x: `https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json`
- A fixed version: `https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0.1.0/dist/questions.json`
- One category: `.../dist/categories/icebreakers.json`

The same files are attached to each [release](https://github.com/kashifkhan0771/cuppa/releases).
The layout is described in [schema/dataset.schema.json](schema/dataset.schema.json);
new fields may be added, so ignore fields you don't know.

## Upgrading

| You use | To upgrade |
|---|---|
| GitHub Action `@v0` | nothing: `@v0` always points to the newest 0.x release |
| Docker `:0` or `:latest` | `docker pull ghcr.io/kashifkhan0771/cuppa:0` |
| A downloaded binary | download the new one from the releases page |
| `go install` | run the same command again |
| The CDN JSON `@v0` | nothing |

Question IDs never change or get reused, so saved histories keep working.

## Development

Requires Go (version in `go.mod`).

```sh
make check              # vet, lint, tests with coverage, dash check
make check-data         # check the question files
make build-data         # rebuild dist/ after editing questions/
make release-snapshot   # build all release files into build/release
```

## License

- Code: [Apache-2.0](LICENSE)
- Questions in `questions/`: [CC0-1.0](questions/LICENSE)
