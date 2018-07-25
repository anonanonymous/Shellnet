const THRESHOLD = 5;
function httpGet(theUrl) {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "/wallet", false);
    xmlHttp.send( null );
    return JSON.parse(xmlHttp.responseText);
}
let availableBalance = httpGet('/wallet').Balance.result.availableBalance;
let blockCount = httpGet('/wallet').Status.result.blockCount;
let knownBlockCount = httpGet('/wallet').Status.result.knownBlockCount;

document.getElementById("wallet_balance").textContent = availableBalance;
document.getElementById("block_count").textContent = blockCount;

if ((knownBlockCount - blockCount < THRESHOLD) && (blockCount > 1)) {
  document.getElementById("wallet_status").classList.add("green-input");
} else {
  document.getElementById("wallet_status").classList.add("orange-input");
}