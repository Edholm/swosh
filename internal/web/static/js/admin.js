// The admin pages: filtering the list of links, and deleting links, one at a
// time or the checked ones together.
document.addEventListener('DOMContentLoaded', function () {
  setUpFilter();
  const deleteDialog = setUpDelete();
  setUpBulk(deleteDialog);
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

// setUpDelete wires the delete dialog and returns a function that opens it
// for a list of ids.
function setUpDelete() {
  const dialog = document.getElementById('delete-dialog');
  const confirmBtn = document.getElementById('delete-confirm');
  const error = document.getElementById('delete-error');
  let ids = [];

  const open = function (toDelete) {
    ids = toDelete;
    const what = document.getElementById('delete-what');
    if (ids.length === 1) {
      what.replaceChildren(document.createElement('code'));
      what.firstChild.textContent = ids[0];
      document.getElementById('delete-text').textContent = "The link stops working right away. This can't be undone.";
    } else {
      what.textContent = ids.length + ' links';
      document.getElementById('delete-text').textContent = "They stop working right away. This can't be undone.";
    }
    error.hidden = true;
    confirmBtn.disabled = false;
    dialog.showModal();
  };

  for (const btn of document.querySelectorAll('button.delete-btn')) {
    btn.addEventListener('click', function (event) {
      // currentTarget, not target: a click on the icon inside the button has
      // the icon as its target.
      open([event.currentTarget.dataset.id]);
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
      let resp;
      if (ids.length === 1) {
        resp = await fetch('/admin/' + encodeURIComponent(ids[0]), {method: 'DELETE', headers: headers});
      } else {
        headers['Content-Type'] = 'application/json';
        resp = await fetch('/admin', {method: 'DELETE', headers: headers, body: JSON.stringify({ids: ids})});
      }
      if (!resp.ok) {
        throw new Error(resp.status + ' ' + resp.statusText);
      }
      // The list, also when deleting from a link's own page.
      location.href = '/admin';
    } catch (err) {
      const what = ids.length === 1 ? ids[0] : ids.length + ' links';
      document.getElementById('delete-error-text').textContent = 'Could not delete ' + what + ': ' + err.message;
      error.hidden = false;
      confirmBtn.disabled = false;
    }
  });

  return open;
}

function setUpBulk(openDelete) {
  const bar = document.getElementById('bulk-bar');
  if (!bar) {
    return;
  }
  const boxes = Array.from(document.querySelectorAll('input.link-select'));
  // Links hidden by the filter can't be seen, so they are never deleted.
  const visible = function (box) {
    return !box.closest('li').hidden;
  };
  const checked = function () {
    return boxes.filter(function (box) {
      return box.checked && visible(box);
    });
  };
  const update = function () {
    const n = checked().length;
    document.getElementById('bulk-count').textContent = n;
    bar.hidden = n === 0;
  };

  for (const box of boxes) {
    box.addEventListener('change', update);
  }
  document.getElementById('filter').addEventListener('input', update);
  document.getElementById('bulk-all').addEventListener('click', function () {
    for (const box of boxes) {
      box.checked = box.checked || visible(box);
    }
    update();
  });
  document.getElementById('bulk-clear').addEventListener('click', function () {
    for (const box of boxes) {
      box.checked = false;
    }
    update();
  });
  document.getElementById('bulk-delete').addEventListener('click', function () {
    openDelete(checked().map(function (box) {
      return box.value;
    }));
  });
  // Back/forward cache and form restoration can bring boxes back checked.
  update();
}
