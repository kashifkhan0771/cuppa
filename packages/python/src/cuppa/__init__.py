"""Watercooler questions: filter them, then pick one at random,
as the question of the day, or without repeats.

Standard library only. The built-in questions are the same ones as in
the Go and npm versions, and daily() gives the same answer in all three.
"""

from __future__ import annotations

import hashlib
import json
import random as _random
from dataclasses import dataclass, field, replace
from datetime import date, datetime
from importlib import resources
from typing import Dict, Iterable, List, Optional, Sequence, Tuple

__all__ = [
    "SCHEMA_VERSION",
    "Category",
    "Dataset",
    "Filter",
    "MemoryHistory",
    "NoMatchError",
    "Question",
    "daily",
    "embedded",
    "next_question",
    "random_question",
]

# The shape version of questions.json this package reads.
SCHEMA_VERSION = 1


class NoMatchError(Exception):
    """There is no question to pick from, usually because the filter left nothing."""

    def __init__(self) -> None:
        super().__init__("cuppa: no questions match the filter")


@dataclass(frozen=True)
class Question:
    id: str
    category: str
    text: str
    type: str
    sfw: str
    locale: str
    added: str
    options: Tuple[str, ...] = ()
    tags: Tuple[str, ...] = ()
    author: str = ""


@dataclass(frozen=True)
class Category:
    id: str
    name: str
    count: int


@dataclass(frozen=True)
class Filter:
    """Which questions to keep. Empty fields mean "no limit", except sfw."""

    categories: Sequence[str] = ()
    tags: Sequence[str] = ()  # keep questions with at least one of these
    exclude_tags: Sequence[str] = ()
    exclude_ids: Sequence[str] = ()
    sfw: str = ""  # "safe" or empty: safe only. "casual": safe and casual
    locale: str = ""
    types: Sequence[str] = ()

    def matches(self, q: Question) -> bool:
        if self.categories and q.category not in self.categories:
            return False
        if self.types and q.type not in self.types:
            return False
        if self.locale and q.locale != self.locale:
            return False
        if q.id in self.exclude_ids:
            return False

        # anything other than "casual" means safe only, the careful default
        if q.sfw != "safe" and self.sfw != "casual":
            return False

        if self.tags and not _has_any(q.tags, self.tags):
            return False
        if self.exclude_tags and _has_any(q.tags, self.exclude_tags):
            return False
        return True


def _has_any(tags: Iterable[str], wanted: Iterable[str]) -> bool:
    return any(tag in wanted for tag in tags)


class Dataset:
    """A set of questions. It never changes after it is built."""

    def __init__(self, file: dict) -> None:
        """Builds a dataset from a parsed questions.json."""
        if file.get("schema_version") != SCHEMA_VERSION:
            raise ValueError(
                "cuppa: questions file has schema_version %r, this version of cuppa reads %d; "
                "try upgrading cuppa" % (file.get("schema_version"), SCHEMA_VERSION)
            )

        questions: List[Question] = []
        seen = set()
        for raw in file["questions"]:
            if raw["id"] in seen:
                raise ValueError('cuppa: question id "%s" appears more than once' % raw["id"])
            seen.add(raw["id"])
            questions.append(_question_from_json(raw))

        names = {c["id"]: c["name"] for c in file.get("categories", [])}
        self._init(file.get("dataset_version", ""), questions, names)

    def _init(self, version: str, questions: Iterable[Question], names: Dict[str, str]) -> None:
        self.version = version
        self._questions = sorted(questions, key=lambda q: q.id)
        self._by_id = {q.id: q for q in self._questions}
        self._names = names

    @classmethod
    def load(cls, path: str) -> "Dataset":
        """Reads a questions.json file from disk."""
        with open(path, encoding="utf-8") as f:
            return cls(json.load(f))

    def __len__(self) -> int:
        return len(self._questions)

    def filter(self, f: Filter) -> "Dataset":
        """Returns a new dataset with only the questions that match."""
        out = Dataset.__new__(Dataset)
        out._init(self.version, [q for q in self._questions if f.matches(q)], self._names)
        return out

    def get(self, id: str) -> Optional[Question]:
        return self._by_id.get(id)

    def all(self) -> List[Question]:
        """Every question, sorted by id."""
        return list(self._questions)

    def categories(self) -> List[Category]:
        """Categories with at least one question here, with counts, sorted by id."""
        counts: Dict[str, int] = {}
        for q in self._questions:
            counts[q.category] = counts.get(q.category, 0) + 1
        return [Category(id, self._names.get(id) or id, counts[id]) for id in sorted(counts)]


def _question_from_json(raw: dict) -> Question:
    return Question(
        id=raw["id"],
        category=raw["category"],
        text=raw["text"],
        type=raw["type"],
        sfw=raw["sfw"],
        locale=raw["locale"],
        added=raw["added"],
        options=tuple(raw.get("options") or ()),
        tags=tuple(raw.get("tags") or ()),
        author=raw.get("author", ""),
    )


_embedded: Optional[Dataset] = None


def embedded() -> Dataset:
    """The questions built into this version of the package."""
    global _embedded
    if _embedded is None:
        text = resources.files("cuppa").joinpath("questions.json").read_text(encoding="utf-8")
        _embedded = Dataset(json.loads(text))
    return _embedded


def random_question(d: Dataset, rng: Optional[_random.Random] = None) -> Question:
    """Picks any question, each equally likely."""
    if len(d) == 0:
        raise NoMatchError()
    return (rng or _random).choice(d.all())


def _date_text(day) -> str:
    # a datetime is read in its own time zone, like the Go version
    if isinstance(day, datetime):
        return day.date().isoformat()
    if isinstance(day, date):
        return day.isoformat()
    return day  # already "YYYY-MM-DD"


def daily(d: Dataset, seed: str, day) -> Question:
    """Picks the question of the day. It needs no saved state: the same seed
    and date give the same question on every machine, and in the Go and npm
    versions too. day is "YYYY-MM-DD", a date, or a datetime (its own date).
    """
    if len(d) == 0:
        raise NoMatchError()

    # hash "seed|date|id" for every question; the smallest hash wins
    date_text = _date_text(day)

    def hash_of(q: Question) -> bytes:
        return hashlib.sha256(("%s|%s|%s" % (seed, date_text, q.id)).encode("utf-8")).digest()

    return min(d.all(), key=hash_of)


class MemoryHistory:
    """Keeps history in memory, so it is lost when the program stops.

    Any object with used(), mark_used(id, at) and reset() works as a history.
    """

    def __init__(self) -> None:
        self._used: Dict[str, datetime] = {}

    def used(self) -> Dict[str, datetime]:
        return dict(self._used)

    def mark_used(self, id: str, at: datetime) -> None:
        self._used[id] = at

    def reset(self) -> None:
        self._used = {}


def next_question(d: Dataset, history, rng: Optional[_random.Random] = None) -> Question:
    """Picks a question not used yet and marks it as used in the history.
    When every question has been used, it resets the history and starts over.
    """
    if len(d) == 0:
        raise NoMatchError()

    used = history.used()
    unused = [q for q in d.all() if q.id not in used]

    # everything was used: start a new cycle with the full set
    if not unused:
        history.reset()
        unused = d.all()

    q = (rng or _random).choice(unused)
    history.mark_used(q.id, datetime.now())
    return q
