"""Use cuppa from Python 3.9+. The package lives in packages/python.

Run: python daily.py
"""
from datetime import date

import cuppa

# the built-in questions, narrowed to two categories
questions = cuppa.embedded().filter(cuppa.Filter(categories=["icebreakers", "this-or-that"], sfw="casual"))

print("random:", cuppa.random_question(questions).text)

# the question of the day: same answer on every machine, no saved state
print("today: ", cuppa.daily(questions, "my-team", date.today()).text)

# no repeats until every question was used; write your own history to keep it on disk
history = cuppa.MemoryHistory()
for _ in range(3):
    print("next:  ", cuppa.next_question(questions, history).text)
