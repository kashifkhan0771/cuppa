// Use the public questions.json from Node 18+ with no packages.
// Run: node cdn.mjs
const url = "https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json";
const data = await (await fetch(url)).json();

// keep work-safe questions, then pick one
const safe = data.questions.filter((q) => q.sfw === "safe");
console.log(safe[Math.floor(Math.random() * safe.length)].text);
