function generateLink() {
  document.getElementById("erroralert").classList.add("hidden");
  document.getElementById("cardlinkdiv").classList.add("hidden");

  var phone = document.getElementById("phone").value;
  var amount = document.getElementById("amount").value;
  var msg = document.getElementById("message").value;
  var expire = document.getElementById("expire").value;

  var dto = {phone: phone, amount: amount, message: msg, expireAfterSeconds: expire};
  var http = new XMLHttpRequest();
  http.open("POST", "/api/create", true);
  http.setRequestHeader("Content-Type", "application/json; charset=UTF-8");
  http.onreadystatechange = function () {
    if (http.readyState === 4) {
      var resp = JSON.parse(http.responseText);
      if (http.status === 200) {
        document.getElementById("cardlinkdiv").classList.remove("hidden");

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
}
