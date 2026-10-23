document.addEventListener('DOMContentLoaded', function () {
  const buttonElements = document.querySelectorAll('button.delete-btn');
  for (let i = 0; i < buttonElements.length; i++) {
    buttonElements[i].addEventListener('click', deleteSwosh);
  }
});

function deleteSwosh(event) {
  // currentTarget, not target: a click on the icon inside the button has the
  // <i> as its target.
  const swoshId = event.currentTarget.dataset.id;
  if (!confirm("Are you sure you want to delete " + swoshId + "?")) {
    return;
  }
  const headers = {};
  const token = document.querySelector('meta[name="csrf-token"]');
  if (token) {
    headers['X-CSRF-Token'] = token.content;
  }
  fetch("/admin/" + encodeURIComponent(swoshId), {method: 'DELETE', headers: headers})
    .then(function (resp) {
      if (!resp.ok) {
        throw new Error(resp.status + " " + resp.statusText);
      }
      // The list, also when deleting from a link's own page.
      location.href = "/admin";
    })
    .catch(function (err) {
      alert("Could not delete " + swoshId + ": " + err.message);
    });
}
