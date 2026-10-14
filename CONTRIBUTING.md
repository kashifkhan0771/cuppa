# Contributing to cuppa

Thank you for helping. There are two ways to contribute, and the first one
needs no Go.

By taking part you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
Security problems go to [SECURITY.md](SECURITY.md), not to a public issue.

## 1. Add or fix questions

1. Read [QUESTION_GUIDELINES.md](QUESTION_GUIDELINES.md).
2. Edit a file in [questions/](questions/) (one file per category). Use the
   next free ID in that file.
3. Open a pull request. The "Questions follow the rules" check tells you what
   to fix, and shows added, changed and removed questions on the run page.

If you have Go installed you can check first with `make check-data`.
You do not need to run `make build-data`: a maintainer does that, and CI
tells you if `dist/` is out of date.

Questions are CC0 (public domain). By adding one you agree to that.

## 2. Change the code

You need Go (the version is in `go.mod`). Node 18+ and Python 3.9+ are only
needed for the packages in `packages/`.

```sh
make check              # vet, lint, tests with coverage, dash check
make test               # tests with the race detector
make check-data         # check the question files
make build-data         # rebuild dist/ after editing questions/
make sync-packages      # copy dist/ into the npm and Python packages
make test-packages      # run the npm and Python tests
make release-snapshot   # build all release files into build/release
```

How the code is laid out:

| Folder | What lives there |
|---|---|
| `pkg/cuppa` | the public Go library: filter, random, daily, no-repeat. No network calls, no third party packages. |
| `pkg/adapters` | one folder per chat app |
| `internal/cli` | the commands, including `serve` and its OpenAPI file |
| `internal/config` | reading and checking `cuppa.yaml` |
| `internal/history`, `internal/scheduler`, `internal/selfupdate` | no-repeat history, cron, updates |
| `internal/tools/datagen` | turns `questions/` into `dist/` and checks the rules |
| `packages/npm`, `packages/python` | the JS and Python ports of the library |
| `examples/` | ready-to-copy setups |

### Rules we keep

- **Keep it small.** Prefer the standard library. A new dependency needs a
  good reason in the pull request.
- **Every new config key** gets a default, an entry in `examples/cuppa.yaml`
  and the README, and a line in `internal/config/testdata/v1-full.yaml`.
- **Every new chat app** gets a golden test of the exact payload it sends
  (see `pkg/adapters`), a row in the README table and an example config.
- **Never log** webhook URLs or tokens.
- **The compatibility promise** in the README is tested. Config keys are
  only added, `/v1` fields stay, question IDs are never reused. If a test
  that protects the promise fails, change your code, not the test.
- **No long dashes.** Do not use the en dash or em dash anywhere (code,
  comments, docs, commit messages). Use a hyphen, colon, comma or "to".
  `make check-dashes` fails if one slips in.
- **`daily()` must give the same answer in Go, JS and Python.** All three run
  `testdata/daily_vectors.json`.

### Pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/) for the
  title, like `feat(adapters): add Zulip` or `fix(cli): handle empty config`.
  The release notes and version numbers are made from them.
- Keep a pull request to one change. Tests come with the code.
- `make check` must pass. CI runs the same checks and more (Windows and
  macOS tests, `govulncheck`, a release build, the GitHub Action dry run).

## Releases

Maintainers only. release-please keeps a release pull request open. Merging
it creates the tag, and the Release workflow publishes binaries, the Docker
image and the npm and PyPI packages.
