# cuppa

*Grab a cuppa. Answer today's question.*

A **cuppa** is British slang for a cup of tea: the short break where people
chat about nothing in particular. cuppa brings that break to your team chat.
Every day it posts one friendly question ("Coffee or tea?", "What is a skill
you would love to learn?") to Slack, Google Chat, Microsoft Teams, Discord,
Mattermost, Webex, or any service that accepts a webhook.

There is no central server and nothing to sign up for. The questions are plain
data in this repository, and you run cuppa wherever you like: GitHub Actions,
cron, Docker, Kubernetes, or your laptop.

## What you get

- **About 150 ready-made questions** in five categories: icebreakers, would
  you rather, this or that, fun facts about you, and hypotheticals. Every one
  is checked for inclusiveness, duplicates and blocked words.
- **Three kinds of question:** open questions, polls (numbered options), and
  this-or-that.
- **Safe by default.** Questions are `safe` (fit any workplace) or `casual`
  (lighter, for relaxed teams). You only get `safe` unless you ask for more.
- **Seven chat targets:** Slack, Google Chat, Teams, Discord, Mattermost,
  Webex and a generic webhook. Post to several at once.
- **Three ways to pick:** the same question of the day for everyone with no
  saved state, any random question, or never repeating until every question
  was used.
- **Four ways to run it:** a GitHub Action, one command from cron or a
  scheduler, a built-in daemon with its own cron schedule, or an HTTP API
  you host yourself.
- **Your own questions.** Add a private pack next to the built-in ones.
- **Use it from code.** A Go library, plus JavaScript and Python versions
  that pick the same question of the day.
- **Static JSON on a CDN** for scripts, spreadsheets, Google Apps Script and
  Power Automate.
- **Small and careful.** One binary, no runtime. Webhook URLs and tokens are
  never logged, and cuppa warns if you paste one into a config file.

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

Run it once from the Actions tab to see it work.

### Action inputs

| Input | What it does |
|---|---|
| `slack_webhook`, `gchat_webhook`, `teams_webhook`, `discord_webhook`, `mattermost_webhook` | the webhook of that chat app. Pass them from secrets. Set several to post to several. |
| `webex_token`, `webex_room_id` | Webex bot token and the space's room ID (use both) |
| `webhook_url` | any other webhook; receives `{"text": "..."}` as JSON |
| `mode` | `daily` (default), `random` or `no-repeat` |
| `seed` | daily mode: teams with different seeds get different questions (default: the repository name) |
| `categories` | comma separated, like `icebreakers,this-or-that` |
| `sfw` | `safe` (default) or `casual` |
| `config` | path to your own `cuppa.yaml`. It then decides everything (targets, categories, messages, private packs) |
| `dry_run` | `true` picks and prints the question without posting |

Outputs: `question_id` and `question_text`. More workflows (many chats,
your own config, no-repeat with a cache) are in [examples/github-actions](examples/github-actions).

## Install the command line tool

- **Download:** binaries for Linux, macOS and Windows (amd64 and arm64) are on
  the [releases page](https://github.com/kashifkhan0771/cuppa/releases), with
  a `checksums.txt` to verify them.
- **Go:** `go install github.com/kashifkhan0771/cuppa/cmd/cuppa@latest`
- **Docker:** `docker run --rm ghcr.io/kashifkhan0771/cuppa next`

Then:

```sh
cuppa init                 # answer 3 questions, get a cuppa.yaml
export SLACK_WEBHOOK='https://hooks.slack.com/services/...'
cuppa post --dry-run       # see what would be posted
cuppa post                 # post it
```

## Commands

```
cuppa [global flags] <command> [flags]
```

Global flags work before or after the command:

| Flag | Meaning |
|---|---|
| `--config path` | the config file. Without it cuppa uses `./cuppa.yaml`, then your user config folder, then `~/.config/cuppa/cuppa.yaml`. Commands that only read questions (`next`, `list`, `categories`) also work with no config at all. |
| `--log-level level` | `debug`, `info` (default), `warn` or `error` |
| `--log-format format` | `text` (default) or `json` |

| Command | What it does |
|---|---|
| [`init`](#init) | create a `cuppa.yaml` by answering a few questions |
| [`next`](#next) | print a question without posting it |
| [`post`](#post) | pick a question and post it to the chat targets |
| [`daemon`](#daemon) | keep running and post on a cron schedule |
| [`serve`](#serve) | run the HTTP API |
| [`list`](#list-and-categories) | list questions |
| [`categories`](#list-and-categories) | list categories with counts |
| [`validate`](#validate) | check a config or a question pack |
| [`history`](#history) | show or reset the no-repeat history |
| [`version`](#version-and-self-update) | print version information, optionally check for a newer one |
| [`self-update`](#version-and-self-update) | replace this cuppa with the newest release |
| `help` | show the command list; `cuppa <command> --help` shows a command's flags |

**Exit codes:** `0` ok, `1` failed, `2` bad config, flags or usage, `3`
posting failed on at least one target, `4` no question matches the filter.

### init

`cuppa init [--output cuppa.yaml] [--force]`

Asks which chat app you use, which environment variable will hold its webhook
(or token), and how to pick questions, then writes a starter config and tells
you the next steps. It never writes a secret into the file. It refuses to
overwrite an existing file unless you pass `--force`.

### next

`cuppa next [--category a,b] [--tag x,y] [--mode daily|random|no-repeat] [--json]`

Prints the message for one question, the way it would be posted, and posts
nothing. `--json` prints the question itself. The flags narrow the config's
selection for this run. In `no-repeat` mode it counts the question as used.

### post

`cuppa post [--target name] [--dry-run] [--json] [--category a,b] [--tag x,y] [--mode m]`

Picks a question and posts it to every target in the config. Targets are
tried one after another, and a failure on one does not stop the others.

- `--target name` posts to that target only.
- `--dry-run` shows what would be posted. Nothing is sent and no history is
  saved.
- `--json` prints the posted question as JSON (the GitHub Action uses this).
- In `no-repeat` mode the question is only counted as used if at least one
  target got it.
- Exit code `3` means at least one target failed.

### daemon

`cuppa daemon [--dry-run]`

Keeps running and posts on the schedule in the config, so you do not need
cron or CI:

```yaml
schedule:
  cron: "0 10 * * 1-5"      # 10:00, Monday to Friday
  timezone: Asia/Karachi    # empty means the computer's own
  skip_weekends: false
  skip_dates: ["2026-12-25"]
```

The cron line is five fields (minute, hour, day of month, month, day of
week) and takes numbers, `*`, ranges, steps and lists, but not names like
`MON` or `JAN`. If the computer was asleep or off at post time and wakes up
more than an hour late, that post is skipped instead of sent late. A failed
post is logged and the daemon keeps going. Before each post it picks up
refreshed questions (see `data.refresh`). It stops cleanly on Ctrl+C or
SIGTERM. `--dry-run` logs what it would post. Systemd, Docker and Compose
setups are in [examples](examples).

### serve

`cuppa serve [--addr host:port] [--with-scheduler] [--insecure-allow-post]`

Runs the [HTTP API](#http-api) described below. It uses the same config,
private packs and history as the other commands.

- `--addr` sets the address (default `127.0.0.1:8080`, or `server.addr`).
  The default only lets this computer in. cuppa warns if you listen on other
  addresses without a `server.token`.
- `--with-scheduler` also posts on `schedule.cron`, like `daemon`.
- `--insecure-allow-post` allows `POST /v1/post` when no token is set.
  Without a token, posting is off.

### list and categories

`cuppa list [--category a,b] [--json]` prints one line per question: ID,
category and text. It lists what the config's selection allows (so `casual`
questions only show with `sfw: casual`), including your private packs.

`cuppa categories [--json]` prints every category with its question count.

### validate

`cuppa validate [path]`

With no path, checks the config: it parses, secrets are filled in, the
templates compile, the time zone exists, the targets are complete and at
least one question matches the selection. It then prints how many questions
match and, if a schedule is set, when the daemon posts next. With a path to a YAML file, it checks that question pack against
the same rules as the built-in questions.

### history

`cuppa history [show|reset]`

`show` (the default) prints where the history file is, how many questions
were used in this cycle and how many cycles finished. `reset` forgets it and
starts a new cycle.

### version and self-update

`cuppa version [--check]` prints the cuppa version, the dataset version and
how many questions are built in. `--check` asks GitHub if a newer release
exists. cuppa never checks on its own.

`cuppa self-update [--yes]` replaces this binary with the newest release of
the same major version. It shows the old and new versions and asks first
(`--yes` skips that), and checks the download against the release's
`checksums.txt` before replacing anything. A newer major version is only
mentioned, because it may need config changes. It refuses to run for a
Homebrew install or a build from source.

## How questions are picked

Set `selection.mode` in the config, or pass `--mode`.

| Mode | Behavior |
|---|---|
| `daily` | The same question for everyone with the same `seed` on the same day, with **no saved state**. It hashes `seed|date|question id` and takes the smallest hash, so it gives the same answer on every machine and in the Go, JavaScript and Python versions. The day changes at midnight in `schedule.timezone`. Questions can repeat over the months. |
| `random` | Any question each time. |
| `no-repeat` | Never repeats until every question was used, then starts a new cycle. It keeps a small history file (`history.path`) with a lock, so several processes can share it. New questions from an upgrade join the pool by themselves. |

Narrow the pool with `categories`, `tags`, `exclude_tags`, `sfw` and `locale`.

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
Bold text differs per app, and cuppa writes it correctly for each one.
A config file per app, and one for many at once, is in
[examples/chat-apps](examples/chat-apps).

## Config

`cuppa init` writes a starter file. Every option, with comments, is in
[examples/cuppa.yaml](examples/cuppa.yaml). All keys are optional except
`targets`, and `targets` only matters for `post`, `daemon` and `serve`.
Secrets go in environment variables: write `webhook: ${SLACK_WEBHOOK}`.

| Key | Default | What it does |
|---|---|---|
| `version` | `1` | the config layout. A file with a newer number is rejected with a hint to upgrade. |
| `schedule.cron` | none | when `daemon` posts |
| `schedule.timezone` | the computer's | the clock for the schedule and for the day change in `daily` mode |
| `schedule.skip_weekends` | `false` | no post on Saturday or Sunday |
| `schedule.skip_dates` | none | no post on these days (`YYYY-MM-DD`) |
| `selection.mode` | `daily` | `daily`, `random` or `no-repeat` |
| `selection.seed` | `cuppa` | `daily` mode: the same seed gives the same question |
| `selection.categories` | all | only these categories |
| `selection.tags`, `selection.exclude_tags` | none | keep questions with one of these tags; drop questions with any of these |
| `selection.sfw` | `safe` | `safe`, or `casual` to add lighter questions |
| `selection.locale` | `en` | question language |
| `data.extra_questions` | none | your private question files |
| `data.refresh` | `false` | fetch newer questions from the CDN without upgrading |
| `data.refresh_url` | the CDN | where to fetch them (https only) |
| `data.refresh_interval` | `24h` | at most one download per interval |
| `history.path` | `~/.local/state/cuppa/history.json` | the no-repeat history (`$XDG_STATE_HOME` is honored; on Windows it is under `%LocalAppData%`) |
| `message.template` | see below | the message for open questions |
| `message.poll_template` | see below | the message for polls and this-or-that |
| `targets[]` | none | where to post: `name`, `type`, and `webhook`, or `token` and `room_id` (Webex), or `url`, `method`, `headers`, `body` (webhook) |
| `server.addr` | `127.0.0.1:8080` | where `serve` listens |
| `server.token` | none | the token for `POST /v1/post` and `GET /v1/questions/next` |
| `server.cors_origins` | none | websites allowed to call the API from a browser (`"*"` for any) |

Relative paths in the config (such as `extra_questions`) are read from the
config file's folder. Unknown keys are an error, so typos get caught.

**Messages** are Go templates. `{{ .Bold "text" }}` writes bold for the app
being posted to. The fields are `.Text`, `.ID`, `.Category`, `.Type`, `.Tags`,
`.Options` and `.Emojis` (the numbers 1 to 6 for poll options). The default is:

```
☕ {{ .Bold "Today's watercooler:" }} {{ .Text }}
```

The generic `webhook` target also has `.Rendered` (the finished message),
`.Question` and a `json` function for safe JSON values, for building the body.

**Your own questions.** Put them in a YAML file in the same format as the
files in [questions/](questions/) (see [examples/private-pack](examples/private-pack)),
list the file under `data.extra_questions`, and check it with
`cuppa validate my-pack.yaml`. Use your own ID prefix. If one of your IDs
matches a built-in ID, yours wins and cuppa warns.

## Run on a schedule

| Where | How |
|---|---|
| GitHub | the Action above, with a `schedule:` trigger |
| A server with systemd | a timer that runs `cuppa post`, or a service that runs `cuppa daemon` |
| cron | `0 10 * * 1-5 cuppa post` |
| Windows | Task Scheduler running `cuppa.exe post` |
| Docker | `docker run -d --restart unless-stopped -e SLACK_WEBHOOK -v "$PWD/cuppa.yaml:/work/cuppa.yaml:ro" -v cuppa-home:/home/nonroot ghcr.io/kashifkhan0771/cuppa daemon` (the volume keeps the history) |
| Kubernetes | a CronJob running `cuppa post` |
| Google Apps Script | a time trigger running a small script |

Copy-paste versions of all of these are in [examples](examples).

## HTTP API

`cuppa serve` answers questions over HTTP, for your own bots, intranet pages,
spreadsheets or scripts. No account, no database.

```sh
cuppa serve                      # listens on 127.0.0.1:8080
curl localhost:8080/v1/questions/today
curl 'localhost:8080/v1/questions/random?category=this-or-that'
curl -H "Authorization: Bearer $CUPPA_TOKEN" localhost:8080/v1/questions/next
curl -X POST -H "Authorization: Bearer $CUPPA_TOKEN" localhost:8080/v1/post
```

### Endpoints

| Endpoint | What it does |
|---|---|
| `GET /v1/questions/random` | any question |
| `GET /v1/questions/today` | the question of the day (also takes `seed`) |
| `GET /v1/questions/next` | the next unused question; saves it to the history. Needs the token when `server.token` is set. |
| `GET /v1/questions/{id}` | one question by ID |
| `GET /v1/questions` | a list, paged with `limit` (1 to 1000, default 100) and `offset` |
| `GET /v1/categories` | categories with counts |
| `POST /v1/post` | pick a question and post it to the config's targets, like `cuppa post`. Takes `target` to post to one. Needs the token (or `--insecure-allow-post`). |
| `GET /healthz` | `{"status": "ok"}` while the server is up |
| `GET /readyz` | `ok` once questions are loaded, else 503 |
| `GET /version` | cuppa version, commit, build date and question count |
| `GET /openapi.json` | the [OpenAPI 3.1 description](internal/cli/openapi.json) of this API |

The question endpoints take these filters in the query string, which narrow
the config's selection: `category`, `tag` and `type` (comma separated),
`sfw` (`safe` or `casual`) and `locale`.

### Replies

A question looks like this:

```json
{
  "data": {
    "id": "tot-0001",
    "category": "this-or-that",
    "text": "Coffee or tea?",
    "type": "this-or-that",
    "options": ["Coffee", "Tea"],
    "tags": ["food"],
    "sfw": "safe",
    "locale": "en",
    "added": "2026-10-14",
    "author": "kashifkhan0771"
  },
  "meta": { "dataset_version": "2026.10.14", "api_version": "v1" }
}
```

Lists add `total`, `limit` and `offset` to `meta`. `POST /v1/post` answers
`{"data": {"question": {...}, "posted": 1, "targets": 2}}`: status 200 when at
least one target got it (compare `posted` with `targets`), and 502 when none
did. Errors look like this:

```json
{ "error": { "code": "no_match", "message": "no questions match the filter" } }
```

| Status | `code` | When |
|---|---|---|
| 400 | `bad_request` | a bad filter, `limit` or `offset` |
| 401 | `unauthorized` | the token is missing or wrong |
| 403 | `post_disabled` | posting needs `server.token` (or `--insecure-allow-post`) |
| 404 | `not_found` | no question with that ID, or no target with that name |
| 404 | `no_match` | no question matches the filters |
| 429 | `rate_limited` | more than 10 posts a minute; see `Retry-After` |
| 502 | `post_failed` | no target accepted the post |
| 500 | `config_error`, `internal` | a problem on the server |

New fields may be added to replies in the future, so ignore fields you do
not know. The `/v1` fields shown here will not change or disappear.

### Security

- Set `server.token` (from an environment variable). Then `POST /v1/post` and
  `/v1/questions/next` need `Authorization: Bearer <token>`. Without a token,
  posting is off unless you start with `--insecure-allow-post`.
- Posting is limited to 10 a minute per client address. Behind a reverse
  proxy all clients share one address, so the proxy should limit too.
- Browsers on other websites can only call the API if their origin is in
  `server.cors_origins`.
- To share it, put a reverse proxy with HTTPS in front. With Docker, listen on
  all addresses inside the container and publish the port to this computer only:

```sh
docker run -d -p 127.0.0.1:8080:8080 -e CUPPA_TOKEN \
  -v "$PWD/cuppa.yaml:/work/cuppa.yaml:ro" \
  ghcr.io/kashifkhan0771/cuppa serve --addr 0.0.0.0:8080
```

### Questions as static JSON (no server)

Every release publishes the questions as JSON on a CDN, so scripts,
spreadsheets, Google Apps Script or Power Automate can use them with no
install:

- Latest 0.x: `https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json`
- A fixed version: `https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0.1.0/dist/questions.json`
- One category: `.../dist/categories/icebreakers.json`

The same files are attached to each [release](https://github.com/kashifkhan0771/cuppa/releases).
The layout is described in [schema/dataset.schema.json](schema/dataset.schema.json).
Working snippets for curl, Python, Node, Google Apps Script and Power Automate
are in [examples](examples).

### Libraries

Use the same picks inside your own code:

- **Go:** `go get github.com/kashifkhan0771/cuppa/pkg/cuppa`. It has `Embedded`,
  `Load`, `Merge`, `Filter`, `Random`, `Daily` and `Next`, no network calls,
  and no dependencies.
- **JavaScript / TypeScript** (Node 18+, no dependencies): [packages/npm](packages/npm).
- **Python** (3.9+, standard library only): [packages/python](packages/python).

`Daily` gives the same question for the same seed and date in all three, and
all three run the same test cases ([testdata/daily_vectors.json](testdata/daily_vectors.json)).
See [examples/library](examples/library).

## Upgrading

| You use | To upgrade |
|---|---|
| GitHub Action `@v0` | nothing: `@v0` always points to the newest 0.x release |
| Docker `:0` or `:latest` | `docker pull ghcr.io/kashifkhan0771/cuppa:0` |
| A downloaded binary | `cuppa self-update` (checks the checksum first) |
| `go install` | run the same command again |
| The CDN JSON `@v0` | nothing |
| Just the questions | set `data.refresh: true` in the config |

`cuppa version --check` tells you if a newer release exists; cuppa never
checks on its own. `self-update` stays within your major version, because a
new major may need config changes; it tells you when one is out.

`data.refresh` downloads the newest questions of your major version from the
CDN at most once a day (`data.refresh_interval`), keeps a copy in your cache
folder, and falls back to that copy or the built-in questions when offline.
`cuppa daemon` picks up refreshed questions before each post; `cuppa serve`
loads them when it starts.

## Compatibility promise

Within a major version (for example everything 1.x):

- **Config:** keys are only added. A key that is going away warns for a whole
  major version first. A config that works today keeps working.
- **HTTP API:** `/v1` never breaks. New things arrive as new fields or under
  `/v2`.
- **Question IDs** never change meaning and are never reused. Removed
  questions are kept in [questions/_retired.yaml](questions/_retired.yaml), so
  saved histories keep working.
- **Dataset file:** `schema_version` only changes in a new major version.
- **History file:** it is versioned and upgraded automatically; an older cuppa
  refuses a newer file with a hint to upgrade.
- **`daily` picks** for a given seed and date are stable unless questions are
  added, and then only change if a new question wins.

Tests and CI checks protect these promises: a fixture config with every key,
an old history file, the `/v1` field names, the shared `daily` test cases, and
a check that a removed question must be added to `_retired.yaml`.

## Contributing

Adding questions needs no Go: edit a YAML file and open a pull request. See
[CONTRIBUTING.md](CONTRIBUTING.md) and [QUESTION_GUIDELINES.md](QUESTION_GUIDELINES.md).
Please report security problems privately, as described in [SECURITY.md](SECURITY.md).
Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

- Code: [Apache-2.0](LICENSE)
- Questions in `questions/`: [CC0-1.0](questions/LICENSE)
