// Use cuppa from Node 18+. The package lives in packages/npm.
// Run: node daily.mjs
import { daily, embedded, MemoryHistory, next, random } from "cuppa";

// the built-in questions, narrowed to two categories
const questions = embedded().filter({ categories: ["icebreakers", "this-or-that"], sfw: "casual" });

console.log("random:", random(questions).text);

// the question of the day: same answer on every machine, no saved state
// (a Date is read in the time zone you pass, or this computer's)
console.log("today: ", daily(questions, "my-team", new Date(), "Europe/London").text);

// no repeats until every question was used; write your own History to keep it on disk
const history = new MemoryHistory();
for (let i = 0; i < 3; i++) {
  console.log("next:  ", (await next(questions, history)).text);
}
