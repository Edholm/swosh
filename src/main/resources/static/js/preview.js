document.addEventListener("DOMContentLoaded", function (event) {
  var url = new URL(window.location.href);
  var redirect = url.searchParams.get('redirect');
  var isMobile = window.matchMedia("only screen and (max-width: 750px)").matches;

  if (redirect !== "false" && isMobile) {
    window.location = getSwishUrl();
  }
});

function getSwishUrl() {
  var swishBtn = document.getElementById("openSwish");
  return swishBtn.getAttribute("href");
}
