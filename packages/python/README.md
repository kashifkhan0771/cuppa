# cuppa (Python)

Watercooler questions for team chat. The same questions and the same
`daily()` answer as the [cuppa](https://github.com/kashifkhan0771/cuppa) CLI
and the npm package. Standard library only. Needs Python 3.9 or newer.

```python
import cuppa

questions = cuppa.embedded().filter(cuppa.Filter(categories=["icebreakers"]))

cuppa.random_question(questions).text                 # any question
cuppa.daily(questions, "my-team", "2026-10-14").text  # same on every machine, no saved state

# no repeats until every question was used; bring your own history to keep it on disk
history = cuppa.MemoryHistory()
cuppa.next_question(questions, history).text
```

- `Filter` takes `categories`, `tags`, `exclude_tags`, `exclude_ids`, `sfw` (safe only by default, `"casual"` to include casual questions), `locale` and `types`.
- `daily` takes a `"YYYY-MM-DD"` string, a `date`, or a `datetime` (its own date, in its own time zone).
- A history is any object with `used()`, `mark_used(id, at)` and `reset()`.
- `Dataset(parsed_json)` or `Dataset.load(path)` loads your own `questions.json`.
