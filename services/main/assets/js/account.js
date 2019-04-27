// Forking config.
const tickerSymbol = 'TRTL';
const decimalPlaces = 2;

// Wallet update interval in milliseconds. Probably don't need to change this.
const updateInterval = 5000;

function setWalletStatus () {
    let wallet_info = httpGet("/account/wallet_info");
    let availableBalance = parseFloat(wallet_info.Data.balance.availableBalance).toFixed(decimalPlaces);
    let lockedAmount = parseFloat(wallet_info.Data.balance.lockedAmount).toFixed(decimalPlaces);
    let knownBlockCount = wallet_info.Data.status.knownBlockCount;
    let blockCount = wallet_info.Data.status.blockCount;

    if ((knownBlockCount - blockCount < THRESHOLD) && (blockCount > 1)) {
      document.getElementById("wallet_status").className = "green-input";
    } else {
      document.getElementById("wallet_status").className = "orange-input";
    }

    document.getElementById("available_balance").textContent = `${availableBalance} ${tickerSymbol}`;
    document.getElementById("locked_amount").textContent = `${lockedAmount} ${tickerSymbol}`;
    document.getElementById("block_count").textContent = blockCount + "/" + knownBlockCount;
    console.log("checking wallet...");
  }

function confirmation () {
    let dest = document.getElementById("send_to").value;
    let amount = document.getElementById("send_amount").value;
    let conf_msg = document.getElementById("send_confirmation");
    let sendTo = document.getElementById("send_to").value;
    conf_msg.textContent = `You are sending ${amount} ${tickerSymbol} to: ${sendTo}`;
}

function getUrlVars() {
  let vars = {};
  let parts = window.location.href.replace(/[?&]+([^=&]+)=([^&]*)/gi, (m,key,value) => {
      vars[key] = value;
  });
  return vars;
}

function setPaymentForm() {
  let vals = getUrlVars();
  if (vals.address !== undefined) {
      document.getElementById('send_to').value = vals.address;
  }
  if (vals.amount !== undefined) {
      document.getElementById('send_amount').value = vals.amount;
  }
  if (vals.paymentid !== undefined) {
      document.getElementById('s_paymentid').value = vals.paymentid;
  }
}

window.setInterval(setWalletStatus, updateInterval);
