# Microsoft Power Automate (and Zapier, Make, n8n)

These tools can call a URL on a schedule and send the answer to Teams or
another app. There are two ways to get the question.

## A. Read the public JSON (nothing to run)

1. **Recurrence** trigger: every 1 day (or weekdays), at the hour you want.
2. **HTTP** action: `GET`
   `https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v1/dist/questions.json`
   (add `/dist/categories/icebreakers.json` instead for one category).
3. **Initialize variable** `index` (Integer), value:
   `rand(0, length(body('HTTP')?['questions']))`
4. **Post message in a chat or channel** (Teams), message:
   `body('HTTP')?['questions'][variables('index')]?['text']`

This picks a random question each run. Questions with `"sfw": "casual"` are in
the file too; add a **Filter array** step on `sfw` equal to `safe` if you want
to keep it strictly work-safe.

## B. Call your own `cuppa serve` (same daily question as everyone else)

If cuppa runs on a server (see [../docker](../docker) and
[../cuppa.serve.yaml](../cuppa.serve.yaml)), use its HTTP API instead:

1. **Recurrence** trigger.
2. **HTTP** action: `GET https://cuppa.example.com/v1/questions/today`
3. **Post message**: `body('HTTP')?['data']?['text']`

Or let the server post for you with one call:
`POST https://cuppa.example.com/v1/post` with the header
`Authorization: Bearer <your server.token>`.

## Other tools

- **Zapier / Make / n8n:** schedule trigger, then a webhook or HTTP step to
  the same URLs, then your chat app's step.
- **Teams without a flow tool:** use the `teams` target in cuppa itself, with
  a Workflows webhook. See [../chat-apps/teams.yaml](../chat-apps/teams.yaml).
- **Anything that accepts JSON over HTTP** can receive posts from cuppa's
  generic `webhook` target: [../chat-apps/webhook.yaml](../chat-apps/webhook.yaml).
