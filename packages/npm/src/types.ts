/** A watercooler question, as written in questions.json. */
export interface Question {
  id: string;
  category: string;
  text: string;
  type: string;
  options?: string[];
  tags?: string[];
  sfw: string;
  locale: string;
  added: string;
  author?: string;
}

export interface Category {
  id: string;
  name: string;
  count: number;
}

/** The exact layout of a questions.json file. */
export interface DatasetFile {
  schema_version: number;
  dataset_version: string;
  categories: Category[];
  questions: Question[];
}

/** Which questions to keep. Empty or missing fields mean "no limit", except sfw. */
export interface Filter {
  categories?: string[];
  tags?: string[];
  excludeTags?: string[];
  excludeIds?: string[];
  /** "safe" or missing: safe only. "casual": safe and casual. */
  sfw?: string;
  locale?: string;
  types?: string[];
}

type MaybePromise<T> = T | Promise<T>;

/** Remembers which questions were used, for next(). Methods may be async. */
export interface History {
  used(): MaybePromise<Map<string, Date>>;
  markUsed(id: string, at: Date): MaybePromise<void>;
  reset(): MaybePromise<void>;
}
