// The link form on the front page. It posts to /api/create and shows the new
// link with buttons to copy, share and preview it.
document.addEventListener('DOMContentLoaded', function () {
  const form = document.getElementById('create-form');
  const phone = document.getElementById('phone');
  const amount = document.getElementById('amount');
  const message = document.getElementById('message');
  const expire = document.getElementById('expire');
  const count = document.getElementById('message-count');
  const createBtn = document.getElementById('create-btn');
  const error = document.getElementById('create-error');
  const errorText = document.getElementById('create-error-text');
  const result = document.getElementById('result');
  const resultUrl = document.getElementById('result-url');
  const copyBtn = document.getElementById('copy-btn');
  const shareBtn = document.getElementById('share-btn');
  const previewLink = document.getElementById('preview-link');
  let share = null;
  let copyTimer = 0;

  // People mostly make links to their own number, so remember the last one.
  phone.value = load('swosh.phone');

  function updateCount() {
    count.textContent = message.value.length + '/' + message.maxLength;
  }
  message.addEventListener('input', updateCount);
  updateCount();

  for (const input of [phone, amount, message]) {
    input.addEventListener('input', function () {
      input.removeAttribute('aria-invalid');
    });
  }

  form.addEventListener('submit', async function (event) {
    event.preventDefault();
    hideError();

    const missing = [phone, amount].filter(function (input) {
      return input.value.trim() === '';
    });
    if (missing.length > 0) {
      missing.forEach(markInvalid);
      showError('Fill in the Swish number and the amount.');
      missing[0].focus();
      return;
    }

    const req = {
      phone: phone.value.trim(),
      amount: amount.value.replace(/\s/g, ''),
      message: message.value,
      expireAfterSeconds: expire.value,
    };
    createBtn.disabled = true;
    try {
      const resp = await fetch('/api/create', {
        method: 'POST',
        headers: {'Content-Type': 'application/json; charset=UTF-8'},
        body: JSON.stringify(req),
      });
      const body = await resp.json().catch(function () {
        return {};
      });
      if (!resp.ok) {
        console.log('Error response: ', body);
        showError(body.reason || 'Something went wrong (' + resp.status + ').', true);
        return;
      }
      save('swosh.phone', req.phone);
      showResult(body.url, req);
    } catch (err) {
      console.log('Create failed: ', err);
      showError('Could not reach Swosh. Check your connection and try again.');
    } finally {
      createBtn.disabled = false;
    }
  });

  function showResult(url, req) {
    resultUrl.value = url;
    // Without redirect=false, phones would jump straight into Swish.
    previewLink.href = url + '?redirect=false';
    const text = req.amount + ' kr' + (req.message.trim() ? ': ' + req.message.trim() : '');
    share = {title: 'Swish ' + text, text: 'Swish ' + text, url: url};
    shareBtn.hidden = !(navigator.canShare && navigator.canShare(share));
    setCopied(false);
    result.hidden = false;
    result.scrollIntoView({behavior: 'smooth', block: 'nearest'});
  }

  // showError shows reason and, when it came from the server, marks the
  // field it is about.
  function showError(reason, fromServer) {
    errorText.textContent = reason;
    error.hidden = false;
    result.hidden = true;
    if (fromServer) {
      if (/phone|mobile/i.test(reason)) markInvalid(phone);
      if (/amount/i.test(reason)) markInvalid(amount);
      if (/description/i.test(reason)) markInvalid(message);
    }
    const card = form.closest('.card');
    card.classList.remove('shake');
    void card.offsetWidth; // restart the animation
    card.classList.add('shake');
  }

  function hideError() {
    error.hidden = true;
  }

  function markInvalid(input) {
    input.setAttribute('aria-invalid', 'true');
  }

  copyBtn.addEventListener('click', async function () {
    try {
      await navigator.clipboard.writeText(resultUrl.value);
    } catch {
      // No clipboard API outside secure contexts.
      resultUrl.select();
      document.execCommand('copy');
    }
    setCopied(true);
    clearTimeout(copyTimer);
    copyTimer = setTimeout(setCopied, 2000, false);
  });

  function setCopied(copied) {
    copyBtn.querySelector('use').setAttribute('href', '/img/icons.svg#' + (copied ? 'check' : 'copy'));
    copyBtn.querySelector('span').textContent = copied ? 'Copied!' : 'Copy';
  }

  shareBtn.addEventListener('click', function () {
    navigator.share(share).catch(function () {
      // Cancelled.
    });
  });
});

// load and save use localStorage, which may be missing or full.
function load(key) {
  try {
    return localStorage.getItem(key) || '';
  } catch {
    return '';
  }
}

function save(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Not remembered, then.
  }
}
