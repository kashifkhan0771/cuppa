# cuppa (npm)

Watercooler questions for team chat. The same questions and the same
`daily()` answer as the [cuppa](https://github.com/kashifkhan0771/cuppa) CLI
and the Python package. No dependencies. Needs Node 18 or newer.

```js
import { embedded, random, daily, next, MemoryHistory } from "cuppa";

const questions = embedded().filter({ categories: ["icebreakers"] });

random(questions).text;                         // any question
daily(questions, "my-team", "2026-10-14").text; // same on every machine, no saved state
daily(questions, "my-team", new Date(), "Asia/Karachi"); // a Date, read in a time zone

// no repeats until every question was used; bring your own History to keep it on disk
const history = new MemoryHistory();
(await next(questions, history)).text;
```

- `filter` takes `categories`, `tags`, `excludeTags`, `excludeIds`, `sfw` (`"safe"` by default, `"casual"` to include casual questions), `locale` and `types`.
- A `History` is any object with `used()`, `markUsed(id, at)` and `reset()`. They may be async.
- `new Dataset(json)` loads your own `questions.json`.
- It uses `node:crypto`, so it runs on Node, not in the browser.
