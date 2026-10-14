import { createHash } from "node:crypto";
import { embeddedFile } from "./data.js";
import type { Category, DatasetFile, Filter, History, Question } from "./types.js";

export type { Category, DatasetFile, Filter, History, Question } from "./types.js";

/** The shape version of questions.json this package reads. */
export const SCHEMA_VERSION = 1;

/** Thrown when there is no question to pick from, usually because the filter left nothing. */
export class NoMatchError extends Error {
  constructor() {
    super("cuppa: no questions match the filter");
    this.name = "NoMatchError";
  }
}

// copy the arrays inside a question, so changing the result can't change the dataset
function cloneQuestion(q: Question): Question {
  const copy = { ...q };
  if (q.options) copy.options = [...q.options];
  if (q.tags) copy.tags = [...q.tags];
  return copy;
}

function hasAnyTag(q: Question, tags: string[]): boolean {
  return tags.some((tag) => (q.tags ?? []).includes(tag));
}

function matches(q: Question, f: Filter): boolean {
  if (f.categories?.length && !f.categories.includes(q.category)) return false;
  if (f.types?.length && !f.types.includes(q.type)) return false;
  if (f.locale && q.locale !== f.locale) return false;
  if (f.excludeIds?.includes(q.id)) return false;

  // anything other than "casual" means safe only, the careful default
  if (q.sfw !== "safe" && f.sfw !== "casual") return false;

  if (f.tags?.length && !hasAnyTag(q, f.tags)) return false;
  if (f.excludeTags?.length && hasAnyTag(q, f.excludeTags)) return false;
  return true;
}

/** A set of questions. It never changes after it is built. */
export class Dataset {
  readonly version: string;
  private readonly questions: Question[]; // sorted by id
  private readonly byId: Map<string, Question>;
  private readonly categoryNames: Map<string, string>;

  /** Builds a dataset from a parsed questions.json. */
  constructor(file: DatasetFile) {
    if (file.schema_version !== SCHEMA_VERSION) {
      throw new Error(
        `cuppa: questions file has schema_version ${file.schema_version}, ` +
          `this version of cuppa reads ${SCHEMA_VERSION}; try upgrading cuppa`,
      );
    }
    const seen = new Set<string>();
    for (const q of file.questions) {
      if (seen.has(q.id)) throw new Error(`cuppa: question id "${q.id}" appears more than once`);
      seen.add(q.id);
    }
    const names = new Map(file.categories.map((c) => [c.id, c.name]));
    this.version = file.dataset_version;
    this.questions = sortedById(file.questions);
    this.byId = new Map(this.questions.map((q) => [q.id, q]));
    this.categoryNames = names;
  }

  /** Number of questions. */
  get length(): number {
    return this.questions.length;
  }

  /** Returns a new dataset with only the questions that match the filter. */
  filter(f: Filter): Dataset {
    const kept = this.questions.filter((q) => matches(q, f));
    return new Dataset({
      schema_version: SCHEMA_VERSION,
      dataset_version: this.version,
      categories: [...this.categoryNames].map(([id, name]) => ({ id, name, count: 0 })),
      questions: kept,
    });
  }

  get(id: string): Question | undefined {
    const q = this.byId.get(id);
    return q && cloneQuestion(q);
  }

  /** Every question, sorted by id. */
  all(): Question[] {
    return this.questions.map(cloneQuestion);
  }

  /** Categories that have at least one question here, with counts, sorted by id. */
  categories(): Category[] {
    const counts = new Map<string, number>();
    for (const q of this.questions) counts.set(q.category, (counts.get(q.category) ?? 0) + 1);
    return [...counts]
      .map(([id, count]) => ({ id, name: this.categoryNames.get(id) || id, count }))
      .sort((a, b) => compareText(a.id, b.id));
  }

  // internal: the question at an index, copied
  pick(index: number): Question {
    return cloneQuestion(this.questions[index]);
  }

  // internal: the questions themselves, not copied
  items(): readonly Question[] {
    return this.questions;
  }
}

function compareText(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function sortedById(questions: Question[]): Question[] {
  return [...questions].sort((a, b) => compareText(a.id, b.id));
}

/** The questions built into this version of the package. */
export function embedded(): Dataset {
  return (cachedEmbedded ??= new Dataset(embeddedFile));
}
let cachedEmbedded: Dataset | undefined;

/** A random number from 0 up to but not including 1, like Math.random. */
export type Rng = () => number;

function randomIndex(rng: Rng, n: number): number {
  return Math.floor(rng() * n);
}

/** Picks any question, each equally likely. */
export function random(d: Dataset, rng: Rng = Math.random): Question {
  if (d.length === 0) throw new NoMatchError();
  return d.pick(randomIndex(rng, d.length));
}

// the date as YYYY-MM-DD; a Date uses the given time zone, or this computer's
function dateText(day: string | Date, timeZone?: string): string {
  if (typeof day === "string") return day;
  if (timeZone === undefined) {
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`;
  }
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(day);
  const part = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return `${part("year")}-${part("month")}-${part("day")}`;
}

function dailyHash(seed: string, date: string, id: string): Uint8Array {
  return createHash("sha256").update(`${seed}|${date}|${id}`).digest();
}

function compareBytes(a: Uint8Array, b: Uint8Array): number {
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return a[i] < b[i] ? -1 : 1;
  }
  return 0;
}

/**
 * Picks the question of the day. It needs no saved state: the same seed and
 * date give the same question on every machine, and in the Go and Python
 * versions too. `day` is "YYYY-MM-DD" or a Date; a Date is read in `timeZone`
 * (like "Asia/Karachi"), or in this computer's time zone when that is missing.
 */
export function daily(d: Dataset, seed: string, day: string | Date, timeZone?: string): Question {
  if (d.length === 0) throw new NoMatchError();

  // the question with the smallest hash wins
  const date = dateText(day, timeZone);
  const items = d.items();
  let best = 0;
  let bestHash = dailyHash(seed, date, items[0].id);
  for (let i = 1; i < items.length; i++) {
    const hash = dailyHash(seed, date, items[i].id);
    if (compareBytes(hash, bestHash) < 0) {
      best = i;
      bestHash = hash;
    }
  }
  return d.pick(best);
}

/**
 * Picks a question not used yet and marks it as used in the history.
 * When every question has been used, it resets the history and starts over.
 */
export async function next(d: Dataset, history: History, rng: Rng = Math.random): Promise<Question> {
  if (d.length === 0) throw new NoMatchError();

  const used = await history.used();
  let unused = d.items().filter((q) => !used.has(q.id));

  // everything was used: start a new cycle with the full set
  if (unused.length === 0) {
    await history.reset();
    unused = [...d.items()];
  }

  const q = unused[randomIndex(rng, unused.length)];
  await history.markUsed(q.id, new Date());
  return cloneQuestion(q);
}

/** Keeps history in memory, so it is lost when the program stops. */
export class MemoryHistory implements History {
  private usedAt = new Map<string, Date>();

  used(): Map<string, Date> {
    return new Map(this.usedAt);
  }

  markUsed(id: string, at: Date): void {
    this.usedAt.set(id, at);
  }

  reset(): void {
    this.usedAt = new Map();
  }
}
