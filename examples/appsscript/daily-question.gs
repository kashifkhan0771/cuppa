/**
 * Post the question of the day to Google Chat or Slack from Google Apps Script.
 * No server and no cuppa install: the script reads the public questions.json
 * and picks the same "question of the day" that cuppa itself would pick.
 *
 * Setup (about 3 minutes):
 *   1. Create the chat webhook (Google Chat: space name > Apps & integrations >
 *      Webhooks; Slack: an incoming webhook).
 *   2. Open script.google.com, create a project, paste this file.
 *   3. Project Settings > Script properties > add WEBHOOK_URL (required).
 *      Optional: SEED (default "my-team"), CATEGORIES (like "icebreakers,this-or-that"),
 *      SFW ("safe" or "casual", default "safe").
 *   4. Run postDailyQuestion once and accept the permissions. To test without
 *      posting, run logDailyQuestion.
 *   5. Triggers (clock icon) > Add trigger > postDailyQuestion > Time-driven >
 *      Day timer, then pick the hour. Weekdays only? Use createWeekdayTrigger().
 *
 * The day changes at midnight in the script's time zone
 * (Project Settings > Time zone).
 */

// Latest 0.x questions. Use an exact tag like @v0.1.0 to pin a version.
var QUESTIONS_URL = 'https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v0/dist/questions.json';

function postDailyQuestion() {
  var props = PropertiesService.getScriptProperties();
  var webhook = props.getProperty('WEBHOOK_URL');
  if (!webhook) {
    throw new Error('Set the WEBHOOK_URL script property first');
  }
  var response = UrlFetchApp.fetch(webhook, {
    method: 'post',
    contentType: 'application/json',
    payload: JSON.stringify({ text: buildMessage_(todaysQuestion_()) }),
    muteHttpExceptions: true
  });
  if (response.getResponseCode() >= 300) {
    throw new Error('Chat answered ' + response.getResponseCode() + ': ' + response.getContentText());
  }
}

// Prints today's message in the log without posting it.
function logDailyQuestion() {
  console.log(buildMessage_(todaysQuestion_()));
}

// Runs postDailyQuestion at 10:00 on weekdays. Run this function once.
function createWeekdayTrigger() {
  [ScriptApp.WeekDay.MONDAY, ScriptApp.WeekDay.TUESDAY, ScriptApp.WeekDay.WEDNESDAY,
   ScriptApp.WeekDay.THURSDAY, ScriptApp.WeekDay.FRIDAY].forEach(function (day) {
    ScriptApp.newTrigger('postDailyQuestion').timeBased().onWeekDay(day).atHour(10).create();
  });
}

function todaysQuestion_() {
  var props = PropertiesService.getScriptProperties();
  var seed = props.getProperty('SEED') || 'my-team';
  var sfw = props.getProperty('SFW') || 'safe';
  var categories = (props.getProperty('CATEGORIES') || '')
    .split(',')
    .map(function (c) { return c.trim(); })
    .filter(function (c) { return c; });

  var file = JSON.parse(UrlFetchApp.fetch(QUESTIONS_URL).getContentText());
  var questions = file.questions.filter(function (q) {
    return (q.sfw === 'safe' || sfw === 'casual') &&
      (categories.length === 0 || categories.indexOf(q.category) !== -1);
  });
  if (questions.length === 0) {
    throw new Error('No questions match CATEGORIES and SFW');
  }

  var today = Utilities.formatDate(new Date(), Session.getScriptTimeZone(), 'yyyy-MM-dd');
  return pickDaily_(questions, seed, today);
}

// Same rule as cuppa's daily mode: hash "seed|date|id" with SHA-256 and take
// the question with the smallest hash. Same seed and date, same question,
// in Go, JavaScript, Python and here.
function pickDaily_(questions, seed, date) {
  var best = null;
  var bestHash = null;
  questions.forEach(function (q) {
    var hash = Utilities.computeDigest(
      Utilities.DigestAlgorithm.SHA_256, seed + '|' + date + '|' + q.id, Utilities.Charset.UTF_8);
    if (best === null || compareBytes_(hash, bestHash) < 0) {
      best = q;
      bestHash = hash;
    }
  });
  return best;
}

// Apps Script bytes are signed (-128 to 127); compare them as 0 to 255.
function compareBytes_(a, b) {
  for (var i = 0; i < a.length; i++) {
    var x = a[i] & 0xff;
    var y = b[i] & 0xff;
    if (x !== y) {
      return x < y ? -1 : 1;
    }
  }
  return 0;
}

// Slack and Google Chat both write bold as *text*.
var EMOJIS_ = ['1️⃣', '2️⃣', '3️⃣', '4️⃣', '5️⃣', '6️⃣'];

function buildMessage_(q) {
  var message = '☕ *Today\'s watercooler:* ' + q.text;
  (q.options || []).forEach(function (option, i) {
    message += '\n' + EMOJIS_[i] + ' ' + option;
  });
  return message;
}
