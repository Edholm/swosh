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
  const expireHint = document.getElementById('expire-hint');
  const resultExpiry = document.getElementById('result-expiry');
  const dateFormat = new Intl.DateTimeFormat(undefined, {dateStyle: 'medium', timeStyle: 'short'});
  let share = null;
  let copyTimer = 0;

  // People mostly make links to their own number, so the last few numbers
  // are kept on this device: the latest fills the field and all of them are
  // offered as buttons below it.
  const recent = document.getElementById('recent-numbers');
  const recentList = document.getElementById('recent-list');
  let numbers = loadNumbers();
  phone.value = numbers[0] || '';
  renderNumbers();
  phone.addEventListener('input', markCurrentNumber);

  function renderNumbers() {
    recentList.replaceChildren(...numbers.map(numberChip));
    recent.hidden = numbers.length === 0;
    markCurrentNumber();
  }

  function numberChip(number) {
    const pick = document.createElement('button');
    pick.type = 'button';
    pick.className = 'chip-pick';
    pick.textContent = number;
    pick.dataset.key = numberKey(number);
    pick.addEventListener('click', function () {
      phone.value = number;
      phone.removeAttribute('aria-invalid');
      markCurrentNumber();
      amount.focus();
    });

    const forget = document.createElement('button');
    forget.type = 'button';
    forget.className = 'chip-forget';
    forget.title = 'Forget this number';
    forget.setAttribute('aria-label', 'Forget ' + number);
    forget.append(icon('x'));
    forget.addEventListener('click', function () {
      numbers = numbers.filter(function (n) {
        return numberKey(n) !== numberKey(number);
      });
      saveNumbers(numbers);
      renderNumbers();
      // The button is gone, so focus would fall back to the page.
      phone.focus();
    });

    const chip = document.createElement('li');
    chip.className = 'chip';
    chip.append(pick, forget);
    return chip;
  }

  function markCurrentNumber() {
    const key = numberKey(phone.value);
    for (const pick of recentList.querySelectorAll('.chip-pick')) {
      pick.setAttribute('aria-current', String(pick.dataset.key === key));
    }
  }

  function rememberNumber(number) {
    numbers = [number].concat(numbers.filter(function (n) {
      return numberKey(n) !== numberKey(number);
    })).slice(0, maxNumbers);
    saveNumbers(numbers);
    renderNumbers();
  }

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

  // Only digits and spaces get into the amount, so pasting "150 kr" leaves
  // the number. maxlength fits "150 000".
  const maxAmount = Number(amount.dataset.max);
  amount.addEventListener('input', function () {
    const cleaned = amount.value.replace(/[^\d ]/g, '');
    if (cleaned !== amount.value) {
      amount.value = cleaned;
    }
  });

  // The × in a field empties just that field.
  for (const btn of form.querySelectorAll('[data-clear]')) {
    btn.addEventListener('click', function () {
      const input = document.getElementById(btn.dataset.clear);
      input.value = '';
      input.dispatchEvent(new Event('input', {bubbles: true}));
      input.focus();
    });
  }

  // Clear empties the form and hides the error or link below it, ready for
  // the next link.
  form.addEventListener('reset', function () {
    hideError();
    result.hidden = true;
    for (const input of [phone, amount, message]) {
      input.removeAttribute('aria-invalid');
    }
    // The browser resets the fields after this event, so update what shows
    // their values once it has.
    setTimeout(function () {
      updateCount();
      markCurrentNumber();
      updateExpireHint();
      phone.focus();
    });
  });

  // The expiry sits folded away under Advanced, whose summary tells what it
  // is set to.
  function updateExpireHint() {
    expireHint.textContent = expire.value ? 'Expires after ' + expireLabel() : 'Never expires';
  }
  expire.addEventListener('change', updateExpireHint);
  updateExpireHint();

  function expireLabel() {
    return expire.selectedOptions[0].text;
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
    // The server checks this too; asking here saves a round trip.
    const sum = Number(req.amount);
    if (!Number.isInteger(sum) || sum < 1 || sum > maxAmount) {
      markInvalid(amount);
      showError(sum > maxAmount
        ? 'You can Swish at most ' + maxAmount.toLocaleString('sv-SE') + ' kr.'
        : 'The amount must be a whole number of kronor, at least 1.');
      amount.focus();
      return;
    }

    const expiry = {seconds: Number(expire.value), label: expireLabel()};
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
      rememberNumber(req.phone);
      showResult(body.url, req, expiry);
    } catch (err) {
      console.log('Create failed: ', err);
      showError('Could not reach Swosh. Check your connection and try again.');
    } finally {
      createBtn.disabled = false;
    }
  });

  function showResult(url, req, expiry) {
    resultUrl.value = url;
    // The server sets the expiry from its own clock; this is close enough.
    resultExpiry.textContent = expiry.seconds
      ? 'This link expires in ' + expiry.label + ', on ' +
        dateFormat.format(new Date(Date.now() + expiry.seconds * 1000)) + '.'
      : 'This link never expires.';
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

const numbersKey = 'swosh.numbers';
const maxNumbers = 3;

// numberKey makes "070-123 45 67", "0701234567" and "+46 70 123 45 67" the
// same number.
function numberKey(number) {
  return number.replace(/[^\d+]/g, '').replace(/^(\+|00)46/, '0');
}

// loadNumbers and saveNumbers use localStorage, which may be missing, full
// or hold something else under the key.
function loadNumbers() {
  try {
    const numbers = JSON.parse(localStorage.getItem(numbersKey));
    if (!Array.isArray(numbers)) {
      return [];
    }
    return numbers.filter(function (n) {
      return typeof n === 'string' && n !== '';
    }).slice(0, maxNumbers);
  } catch {
    return [];
  }
}

function saveNumbers(numbers) {
  try {
    localStorage.setItem(numbersKey, JSON.stringify(numbers));
  } catch {
    // Not remembered, then.
  }
}

function icon(name) {
  const ns = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(ns, 'svg');
  svg.setAttribute('class', 'icon');
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS(ns, 'use');
  use.setAttribute('href', '/img/icons.svg#' + name);
  svg.append(use);
  return svg;
}
