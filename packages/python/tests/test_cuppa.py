import json
import unittest
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

import cuppa
from cuppa import Dataset, Filter, MemoryHistory, NoMatchError

VECTORS = Path(__file__).resolve().parents[3] / "testdata" / "daily_vectors.json"


def make_dataset(*questions):
    """A tiny dataset where only the fields we care about are filled in."""
    base = {"category": "icebreakers", "type": "open", "sfw": "safe", "locale": "en", "added": "2026-01-01"}
    return Dataset(
        {
            "schema_version": 1,
            "dataset_version": "v",
            "categories": [{"id": "icebreakers", "name": "Icebreakers", "count": 0}],
            "questions": [{**base, "text": "text " + q["id"], **q} for q in questions],
        }
    )


class CuppaTest(unittest.TestCase):
    # the shared cases every language (Go, JS, Python) must agree on
    def test_daily_matches_shared_vectors(self):
        vectors = json.loads(VECTORS.read_text(encoding="utf-8"))
        d = make_dataset(*[{"id": i} for i in vectors["ids"]])
        for c in vectors["cases"]:
            with self.subTest(seed=c["seed"], date=c["date"]):
                self.assertEqual(cuppa.daily(d, c["seed"], c["date"]).id, c["expected"])

    def test_daily_reads_date_and_datetime(self):
        d = cuppa.embedded()
        want = cuppa.daily(d, "team-alpha", "2026-10-15").id
        self.assertEqual(cuppa.daily(d, "team-alpha", date(2026, 10, 15)).id, want)

        # 02:00 on Oct 15 in Karachi is still Oct 14 in UTC: each uses its own date
        karachi = timezone(timedelta(hours=5))
        moment = datetime(2026, 10, 15, 2, 0, tzinfo=karachi)
        self.assertEqual(cuppa.daily(d, "team-alpha", moment).id, want)
        self.assertEqual(
            cuppa.daily(d, "team-alpha", moment.astimezone(timezone.utc)).id,
            cuppa.daily(d, "team-alpha", "2026-10-14").id,
        )

    def test_embedded_loads_and_filters(self):
        d = cuppa.embedded()
        self.assertGreater(len(d), 0)
        self.assertTrue(d.version)
        casual = d.filter(Filter(categories=["this-or-that"], sfw="casual"))
        self.assertGreater(len(casual), 0)
        self.assertTrue(all(q.category == "this-or-that" for q in casual.all()))
        self.assertEqual([c.id for c in casual.categories()], ["this-or-that"])
        self.assertEqual(casual.categories()[0].name, "This or That")

    def test_filter_rules(self):
        d = make_dataset(
            {"id": "a", "tags": ["fun"]},
            {"id": "b", "tags": ["work"], "type": "poll"},
            {"id": "c", "sfw": "casual"},
            {"id": "d", "locale": "de"},
        )

        def ids(f):
            return [q.id for q in d.filter(f).all()]

        self.assertEqual(ids(Filter()), ["a", "b", "d"])  # casual hidden by default
        self.assertEqual(ids(Filter(sfw="casual")), ["a", "b", "c", "d"])
        self.assertEqual(ids(Filter(tags=["work"])), ["b"])
        self.assertEqual(ids(Filter(exclude_tags=["fun"])), ["b", "d"])
        self.assertEqual(ids(Filter(exclude_ids=["a"])), ["b", "d"])
        self.assertEqual(ids(Filter(types=["poll"])), ["b"])
        self.assertEqual(ids(Filter(locale="de")), ["d"])

    def test_get(self):
        d = make_dataset({"id": "a", "tags": ["fun"]})
        self.assertEqual(d.get("a").tags, ("fun",))
        self.assertIsNone(d.get("missing"))

    def test_empty_dataset_raises(self):
        empty = make_dataset()
        with self.assertRaises(NoMatchError):
            cuppa.random_question(empty)
        with self.assertRaises(NoMatchError):
            cuppa.daily(empty, "s", "2026-01-01")
        with self.assertRaises(NoMatchError):
            cuppa.next_question(empty, MemoryHistory())

    def test_next_never_repeats_until_a_cycle_ends(self):
        d = make_dataset({"id": "a"}, {"id": "b"}, {"id": "c"})
        h = MemoryHistory()
        first = {cuppa.next_question(d, h).id for _ in range(3)}
        self.assertEqual(len(first), 3)

        # fourth pick resets history, so only that pick is used
        cuppa.next_question(d, h)
        self.assertEqual(len(h.used()), 1)

    def test_bad_files_are_rejected(self):
        file = {"schema_version": 1, "dataset_version": "v", "categories": [], "questions": []}
        with self.assertRaisesRegex(ValueError, "schema_version 2"):
            Dataset({**file, "schema_version": 2})
        q = {"id": "a", "category": "c", "text": "t", "type": "open", "sfw": "safe", "locale": "en", "added": "x"}
        with self.assertRaisesRegex(ValueError, "more than once"):
            Dataset({**file, "questions": [q, q]})


if __name__ == "__main__":
    unittest.main()
