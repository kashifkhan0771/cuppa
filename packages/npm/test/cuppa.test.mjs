import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import test from "node:test";
import * as esm from "../dist/esm/index.js";

const cjs = createRequire(import.meta.url)("../dist/cjs/index.js");
const { Dataset, MemoryHistory, NoMatchError, daily, embedded, next, random } = esm;

// a tiny dataset where only the fields we care about are filled in
function makeDataset(questions) {
  return new Dataset({
    schema_version: 1,
    dataset_version: "v",
    categories: [{ id: "icebreakers", name: "Icebreakers", count: 0 }],
    questions: questions.map((q) => ({
      category: "icebreakers",
      text: `text ${q.id}`,
      type: "open",
      sfw: "safe",
      locale: "en",
      added: "2026-01-01",
      ...q,
    })),
  });
}

// the shared cases every language (Go, JS, Python) must agree on
test("daily matches the shared vectors", () => {
  const vectors = JSON.parse(readFileSync(new URL("../../../testdata/daily_vectors.json", import.meta.url)));
  const d = makeDataset(vectors.ids.map((id) => ({ id })));
  for (const impl of [esm, cjs]) {
    for (const c of vectors.cases) {
      assert.equal(impl.daily(d, c.seed, c.date).id, c.expected, `${c.seed} ${c.date}`);
    }
  }
});

test("daily reads a Date in the given time zone", () => {
  const d = embedded();
  // 21:00 UTC on Oct 14 is already Oct 15 in Karachi
  const instant = new Date("2026-10-14T21:00:00Z");
  assert.equal(daily(d, "team-alpha", instant, "UTC").id, daily(d, "team-alpha", "2026-10-14").id);
  assert.equal(daily(d, "team-alpha", instant, "Asia/Karachi").id, daily(d, "team-alpha", "2026-10-15").id);
});

test("embedded dataset loads and filters", () => {
  const d = embedded();
  assert.ok(d.length > 0);
  assert.ok(d.version);
  const casual = d.filter({ categories: ["this-or-that"], sfw: "casual" });
  assert.ok(casual.length > 0);
  assert.ok(casual.all().every((q) => q.category === "this-or-that"));
  assert.deepEqual(casual.categories().map((c) => c.id), ["this-or-that"]);
  assert.equal(casual.categories()[0].name, "This or That");
});

test("filter rules", () => {
  const d = makeDataset([
    { id: "a", tags: ["fun"] },
    { id: "b", tags: ["work"], type: "poll" },
    { id: "c", sfw: "casual" },
    { id: "d", locale: "de" },
  ]);
  const ids = (f) => d.filter(f).all().map((q) => q.id);
  assert.deepEqual(ids({}), ["a", "b", "d"]); // casual hidden by default
  assert.deepEqual(ids({ sfw: "casual" }), ["a", "b", "c", "d"]);
  assert.deepEqual(ids({ tags: ["work"] }), ["b"]);
  assert.deepEqual(ids({ excludeTags: ["fun"] }), ["b", "d"]);
  assert.deepEqual(ids({ excludeIds: ["a"] }), ["b", "d"]);
  assert.deepEqual(ids({ types: ["poll"] }), ["b"]);
  assert.deepEqual(ids({ locale: "de" }), ["d"]);
});

test("results are copies", () => {
  const d = makeDataset([{ id: "a", tags: ["fun"] }]);
  d.get("a").tags.push("changed");
  assert.deepEqual(d.get("a").tags, ["fun"]);
  assert.equal(d.get("missing"), undefined);
});

test("empty dataset throws NoMatchError", async () => {
  const empty = makeDataset([]);
  assert.throws(() => random(empty), NoMatchError);
  assert.throws(() => daily(empty, "s", "2026-01-01"), NoMatchError);
  await assert.rejects(next(empty, new MemoryHistory()), NoMatchError);
});

test("random uses the given rng", () => {
  const d = makeDataset([{ id: "a" }, { id: "b" }, { id: "c" }]);
  assert.equal(random(d, () => 0).id, "a");
  assert.equal(random(d, () => 0.99).id, "c");
});

test("next never repeats until a cycle ends, then starts over", async () => {
  const d = makeDataset([{ id: "a" }, { id: "b" }, { id: "c" }]);
  const h = new MemoryHistory();
  const first = new Set();
  for (let i = 0; i < 3; i++) first.add((await next(d, h)).id);
  assert.equal(first.size, 3);

  // fourth pick resets history, so only that pick is used
  await next(d, h);
  assert.equal(h.used().size, 1);
});

test("next accepts an async history", async () => {
  const d = makeDataset([{ id: "a" }]);
  const inner = new MemoryHistory();
  const asyncHistory = {
    used: async () => inner.used(),
    markUsed: async (id, at) => inner.markUsed(id, at),
    reset: async () => inner.reset(),
  };
  assert.equal((await next(d, asyncHistory)).id, "a");
  assert.equal(inner.used().size, 1);
});

test("a file with the wrong schema version or repeated ids is rejected", () => {
  const file = { schema_version: 1, dataset_version: "v", categories: [], questions: [] };
  assert.throws(() => new Dataset({ ...file, schema_version: 2 }), /schema_version 2/);
  const q = { id: "a", category: "c", text: "t", type: "open", sfw: "safe", locale: "en", added: "x" };
  assert.throws(() => new Dataset({ ...file, questions: [q, q] }), /more than once/);
});
