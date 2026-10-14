# Question guidelines

The questions are plain YAML in [questions/](questions/), released under
CC0 (public domain). A good question is one a whole team can answer in a
chat message, without feeling singled out. `make check-data` and the
"Questions follow the rules" CI check enforce the format rules below.

## What makes a good question

- **Short and fun.** One sentence, 10 to 200 characters, easy to answer in
  one message. It should end with a `?`.
- **Answerable by anyone**, whatever their background, location, income,
  family situation or health.
- **Original.** No quotes from copyrighted works and no trademarked
  characters.
- **Not a near copy** of an existing question. CI rejects close matches and
  names the IDs they collide with.

Good:

- "What is a small thing that always makes your day better?"
- "Would you rather have a 4-day work week or work from anywhere forever?"
- "Coffee or tea?"

Not good, and why:

- "Which party did you vote for?" (politics)
- "What would you do with a million dollars?" (money assumes a lot)
- "Tell us about your family." (not everyone wants to, or can)
- "What is your go-to Super Bowl snack?" (assumes a region)
- "What's your worst hangover story?" (alcohol-centric)

## Topics to avoid

Politics, religion, health and body, money and salary, dating and
relationships, alcohol, trauma, family, and anything that assumes a
country, a holiday or a sports league. If a question is about a place or
season, tag it so teams can filter it out (`seasons`, `weather`).

## The format

```yaml
category: would-you-rather
name: "Would You Rather"
prefix: wyr
questions:
  - id: wyr-0031
    text: "Would you rather have a 4-day work week or work from anywhere forever?"
    type: this-or-that
    options: ["4-day week", "Work from anywhere"]
    tags: [work, remote]
    sfw: safe
    locale: en
    added: 2026-10-14
    author: your-github-handle
```

| Field | Required | Rules |
|---|---|---|
| `id` | yes | `<prefix>-<4 or more digits>`, next free number in the file. Stable forever and never reused. |
| `text` | yes | 10 to 200 characters. |
| `type` | yes | `open`, `poll` or `this-or-that`. |
| `options` | for `poll` and `this-or-that` | 2 to 6 short strings, each up to 40 characters. |
| `tags` | no | From [schema/tags.txt](schema/tags.txt). Add a new tag to that file in the same PR that first uses it. |
| `sfw` | yes | `safe` fits any workplace. `casual` is for relaxed teams. When unsure, pick `casual`. |
| `locale` | yes | Language, like `en`. |
| `added` | yes | The date you add it, `YYYY-MM-DD`. |
| `author` | no | Your GitHub handle. |

Words listed in [schema/blocklist.txt](schema/blocklist.txt) are rejected.

## Changing or removing questions

- Fixing a typo in the text is fine. The ID stays the same.
- Never change what a question means. Add a new question instead.
- To remove a question, delete it and add its ID to
  [questions/_retired.yaml](questions/_retired.yaml) with a reason. CI fails
  if a question disappears without being retired, so IDs are never reused.
