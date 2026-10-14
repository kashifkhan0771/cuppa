# cuppa

*Grab a cuppa. Answer today's question.*

Open-source, self-hostable watercooler questions for work chat: Slack,
Google Chat, Webex, Teams, Discord, Mattermost, or anything with a webhook.
There is no central server. Questions are plain data, and you run cuppa
wherever you like: GitHub Actions, cron, Docker, or your laptop.

> **Status:** early development. Not usable yet.

## Development

Requires Go (version in `go.mod`) and
[golangci-lint](https://golangci-lint.run/) v2.

```sh
make check   # vet, lint, test
make build
```

## License

- Code: [Apache-2.0](LICENSE)
- Questions in `questions/`: [CC0-1.0](questions/LICENSE)
