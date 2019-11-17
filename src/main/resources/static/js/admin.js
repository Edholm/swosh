function deleteSwosh(swoshId) {
  if (confirm("Are you sure you want to delete " + swoshId + "?")) {
    $.ajax("/admin/" + swoshId, {
      method: 'DELETE'
    }).done(function () {
      location.reload();
    });
  }
}
