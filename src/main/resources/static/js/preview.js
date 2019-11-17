document.addEventListener("DOMContentLoaded", function (event) {
  url = new URL(window.location.href);
  var redirect = url.searchParams.get('redirect');
  if (redirect !== "false") {
    window.location = getSwishUrl();
  }
})

function getSwishUrl() {
  var swishBtn = document.getElementById("openSwish");
  return swishBtn.getAttribute("href");
}
