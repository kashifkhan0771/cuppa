"""Use the public questions.json from Python with only the standard library.

Run: python cdn.py
"""
import json
import random
from urllib.request import urlopen

URL = "https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json"

with urlopen(URL, timeout=10) as response:
    data = json.load(response)

# keep work-safe questions from one category
questions = [q for q in data["questions"] if q["sfw"] == "safe" and q["category"] == "icebreakers"]
print(random.choice(questions)["text"])
