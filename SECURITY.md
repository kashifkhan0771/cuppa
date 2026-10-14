# Security policy

## Reporting a problem

Please do not open a public issue for a security problem.

Report it privately: on the repository page, go to **Security > Report a
vulnerability** (GitHub private vulnerability reporting). Include what you
found, how to reproduce it, and which version of cuppa you used.

You will get a first answer within 7 days. Fixes for confirmed problems are
released as patch versions, and the report is credited unless you prefer not.

## Supported versions

Only the newest release in the current major version gets security fixes.
Run `cuppa version --check` to see if you are behind.

## What counts

Examples of things we want to hear about:

- a webhook URL or token showing up in logs, error messages or output
- `cuppa serve` letting someone post or read the history without the token
- `cuppa self-update` or `data.refresh` accepting a file it should reject
- a crafted config, question pack or template that runs code or reads files
- a posted message that can ping people (`@everyone`) when it should not

## Things to know

- Webhook URLs and tokens are secrets. Keep them in environment variables
  and write `${NAME}` in the config. cuppa warns when one is pasted in plain
  text and never logs them.
- `cuppa serve` listens on `127.0.0.1` by default. If you open it to other
  computers, set `server.token` and put it behind HTTPS.
- Releases include a `checksums.txt`. `cuppa self-update` checks it, but it
  comes from the same release, so it catches broken downloads, not a
  compromised release.
