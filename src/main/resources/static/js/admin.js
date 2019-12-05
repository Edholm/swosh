document.addEventListener('DOMContentLoaded', function () {
  const buttonElements = document.querySelectorAll('button.delete-btn');
  for (let i = 0; i < buttonElements.length; i++) {
    buttonElements[i].addEventListener('click', deleteSwosh);
  }
});

function deleteSwosh(event) {
  let swoshId = event.target.id;
  if (confirm("Are you sure you want to delete " + swoshId + "?")) {
    $.ajax("/admin/" + swoshId, {
      method: 'DELETE'
    }).done(function () {
      location.reload();
    });
  }
}
