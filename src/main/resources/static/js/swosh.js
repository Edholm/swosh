document.addEventListener('DOMContentLoaded', function () {
  document.getElementById('generate-link-btn').onclick = generateLink
});

function generateLink() {
  document.getElementById("erroralert").classList.add("hidden");
  document.getElementById("cardlinkdiv").classList.add("hidden");

  const phone = document.getElementById("phone").value;
  const amount = document.getElementById("amount").value;
  const msg = document.getElementById("message").value;
  const expire = document.getElementById("expire").value;

  const dto = {phone: phone, amount: amount, message: msg, expireAfterSeconds: expire};
  const http = new XMLHttpRequest();
  http.open("POST", "/api/create", true);
  http.setRequestHeader("Content-Type", "application/json; charset=UTF-8");
  http.onreadystatechange = function () {
    if (http.readyState === 4) {
      const resp = JSON.parse(http.responseText);
      if (http.status === 200) {
        document.getElementById("cardlinkdiv").classList.remove("hidden");
        document.getElementById("copy-to-clipboard-btn").classList.remove("shake");

        var urlElem = document.getElementById("swoshurl");
        urlElem.href = resp.url;
        urlElem.text = resp.url;
      } else {
        console.log("Error response: ", resp);
        document.getElementById("erroralert").classList.remove("hidden");
        document.getElementById("errortxt").innerHTML = resp.reason
      }
    }
  };
  http.send(JSON.stringify(dto));
}

function copyUrlToClipboard() {
  var aux = document.createElement("input");
  aux.setAttribute("value", document.getElementById("swoshurl").href);
  document.body.appendChild(aux);
  aux.select();
  document.execCommand("copy");
  document.body.removeChild(aux);

  document.getElementById("copy-to-clipboard-btn").classList.add("shake");
}
