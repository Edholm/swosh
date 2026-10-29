// The admin pages: filtering the list of links, and deleting links.
document.addEventListener('DOMContentLoaded', function () {
  setUpFilter();
  setUpDelete();
});

function setUpFilter() {
  const filter = document.getElementById('filter');
  if (!filter) {
    return;
  }
  const items = document.querySelectorAll('#link-list > li');
  const noMatches = document.getElementById('no-matches');
  // Phone numbers are stored as "070-123 45 67"; let "0701234567" find them.
  const normalize = function (s) {
    return s.toLowerCase().replace(/[\s-]/g, '');
  };

  filter.addEventListener('input', function () {
    const q = normalize(filter.value);
    let shown = 0;
    for (const item of items) {
      item.hidden = !normalize(item.dataset.search).includes(q);
      shown += item.hidden ? 0 : 1;
    }
    noMatches.hidden = shown > 0;
  });

  // "/" jumps to the filter, like on GitHub.
  document.addEventListener('keydown', function (event) {
    const target = event.target;
    if (event.key === '/' && !target.closest('input, textarea, select, [contenteditable]')) {
      event.preventDefault();
      filter.focus();
    }
  });
}

function setUpDelete() {
  const dialog = document.getElementById('delete-dialog');
  const confirmBtn = document.getElementById('delete-confirm');
  const error = document.getElementById('delete-error');
  let swoshId = '';

  for (const btn of document.querySelectorAll('button.delete-btn')) {
    btn.addEventListener('click', function (event) {
      // currentTarget, not target: a click on the icon inside the button has
      // the icon as its target.
      swoshId = event.currentTarget.dataset.id;
      document.getElementById('delete-id').textContent = swoshId;
      error.hidden = true;
      confirmBtn.disabled = false;
      dialog.showModal();
    });
  }

  confirmBtn.addEventListener('click', async function () {
    confirmBtn.disabled = true;
    const headers = {};
    const token = document.querySelector('meta[name="csrf-token"]');
    if (token) {
      headers['X-CSRF-Token'] = token.content;
    }
    try {
      const resp = await fetch('/admin/' + encodeURIComponent(swoshId), {method: 'DELETE', headers: headers});
      if (!resp.ok) {
        throw new Error(resp.status + ' ' + resp.statusText);
      }
      // The list, also when deleting from a link's own page.
      location.href = '/admin';
    } catch (err) {
      document.getElementById('delete-error-text').textContent = 'Could not delete ' + swoshId + ': ' + err.message;
      error.hidden = false;
      confirmBtn.disabled = false;
    }
  });
}
