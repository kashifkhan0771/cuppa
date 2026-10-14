/**
 * Get the question of the day from your own "cuppa serve" and post it.
 * Use this when cuppa runs on a server and you want Apps Script (or Google
 * Sheets, Docs or Forms) to use the same questions, config and history.
 *
 * The server must be reachable from the internet over HTTPS (Apps Script runs
 * on Google's servers). Put a reverse proxy in front of "cuppa serve" and
 * set server.token. See ../cuppa.serve.yaml.
 *
 * Script properties (Project Settings):
 *   CUPPA_URL    like https://cuppa.example.com   (no trailing slash)
 *   CUPPA_TOKEN  your server.token (only needed for /next and /post)
 *   WEBHOOK_URL  only for postTodayToChat
 *
 * Add the three kinds of call you need as triggers or call them from sheets.
 */

function cuppa_(path, options) {
  var props = PropertiesService.getScriptProperties();
  var headers = {};
  if (props.getProperty('CUPPA_TOKEN')) {
    headers.Authorization = 'Bearer ' + props.getProperty('CUPPA_TOKEN');
  }
  var response = UrlFetchApp.fetch(props.getProperty('CUPPA_URL') + path, Object.assign({
    headers: headers,
    muteHttpExceptions: true
  }, options || {}));
  var body = JSON.parse(response.getContentText());
  if (response.getResponseCode() >= 300) {
    // errors look like {"error": {"code": "no_match", "message": "..."}}
    throw new Error(body.error.code + ': ' + body.error.message);
  }
  return body;
}

// 1. Read the question of the day. Replies look like {"data": {...}, "meta": {...}}.
function logToday() {
  var question = cuppa_('/v1/questions/today').data;
  console.log(question.id + ': ' + question.text);
}

// 2. Let the server post to the chat apps in its own config. Needs CUPPA_TOKEN.
//    Answers {"data": {"question": {...}, "posted": 1, "targets": 1}, ...};
//    throws when no target got it.
function askServerToPost() {
  var result = cuppa_('/v1/post', { method: 'post' }).data;
  console.log(result.question.id + ' posted to ' + result.posted + ' of ' + result.targets + ' targets');
}

// 3. Read a question and post it to a webhook yourself. Add ?category=... to narrow it.
function postTodayToChat() {
  var question = cuppa_('/v1/questions/today?category=icebreakers,this-or-that').data;
  var webhook = PropertiesService.getScriptProperties().getProperty('WEBHOOK_URL');
  UrlFetchApp.fetch(webhook, {
    method: 'post',
    contentType: 'application/json',
    payload: JSON.stringify({ text: '☕ *Today\'s watercooler:* ' + question.text })
  });
}

// Use it in a sheet cell: =CUPPA_QUESTION()   (a custom function)
function CUPPA_QUESTION() {
  return cuppa_('/v1/questions/random').data.text;
}
