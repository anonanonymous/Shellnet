const THRESHOLD = 5;
function httpGet(theUrl) {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", theUrl, false);
    xmlHttp.send( null );
    return JSON.parse(xmlHttp.responseText);
}

function setWalletStatus() {
  let wallet_info = httpGet("/wallet_info");
  let availableBalance = wallet_info.Data.balance.availableBalance;
  let lockedAmount = wallet_info.Data.balance.lockedAmount;
  let knownBlockCount= wallet_info.Data.status.knownBlockCount;
  let blockCount= wallet_info.Data.status.blockCount;

  if ((knownBlockCount - blockCount < THRESHOLD) && (blockCount > 1)) {
    document.getElementById("wallet_status").className = "green-input";
  } else {
    document.getElementById("wallet_status").className = "orange-input";
  }
  document.getElementById("available_balance").textContent = availableBalance + " TRTL";
  document.getElementById("locked_amount").textContent = lockedAmount + " TRTL";
  document.getElementById("block_count").textContent = blockCount + "/" + knownBlockCount;
  console.log("checking wallet...");
}

function copyAddress() {
  let copyText = document.getElementById("myInput");
  copyText.select();
  document.execCommand("copy");
}

if (window.location.pathname == "/account") {
  window.setInterval(setWalletStatus, 5000);
}