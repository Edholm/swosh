document.addEventListener("DOMContentLoaded", function () {
  const url = new URL(window.location.href);
  const redirect = url.searchParams.get('redirect');
  const isMobile = window.matchMedia("only screen and (max-width: 750px)").matches;

  if (redirect !== "false" && isMobile) {
    window.location = getSwishUrl();
  }
});

function getSwishUrl() {
  const swishBtn = document.getElementById("openSwish");
  return swishBtn.getAttribute("href");
}
