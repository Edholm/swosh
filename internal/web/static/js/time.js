// Shows <time datetime> elements in the reader's time zone and the page's
// language.
// Those marked data-relative read like "in 3 days", the others like
// "22 Oct 2026, 15:00"; the tooltip has the other form. Times in the past get
// data-past.
document.addEventListener('DOMContentLoaded', function () {
  // The page's language, so dates don't come out English on a Swedish page.
  const locale = document.documentElement.lang || undefined;
  const absolute = new Intl.DateTimeFormat(locale, {dateStyle: 'medium', timeStyle: 'short'});
  // "in 1 month", not "next month", which could mean the calendar month.
  const relative = new Intl.RelativeTimeFormat(locale, {numeric: 'always'});
  const units = [
    ['year', 365 * 24 * 3600],
    ['month', 30 * 24 * 3600],
    ['week', 7 * 24 * 3600],
    ['day', 24 * 3600],
    ['hour', 3600],
    ['minute', 60],
    ['second', 1],
  ];

  function fromNow(date) {
    const secs = (date - Date.now()) / 1000;
    // A bit under a whole unit still counts as one, so a link made a minute
    // ago to last a day reads "in 1 day", not "in 24 hours".
    const [unit, size] = units.find(function ([, size]) {
      return Math.abs(secs) >= size * 0.99;
    }) || units[units.length - 1];
    return relative.format(Math.round(secs / size), unit);
  }

  for (const el of document.querySelectorAll('time[datetime]')) {
    const date = new Date(el.dateTime);
    if (isNaN(date)) {
      continue;
    }
    if (date < Date.now()) {
      el.dataset.past = '';
    }
    const [text, title] = 'relative' in el.dataset
      ? [fromNow(date), absolute.format(date)]
      : [absolute.format(date), fromNow(date)];
    el.textContent = text;
    el.title = title;
  }
});
