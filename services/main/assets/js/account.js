function setWalletStatus() {
    let wallet_info = httpGet("/wallet_info");
    let availableBalance = parseFloat(wallet_info.Data.balance.availableBalance);
    let lockedAmount = parseFloat(wallet_info.Data.balance.lockedAmount);
    let knownBlockCount = wallet_info.Data.status.knownBlockCount;
    let blockCount = wallet_info.Data.status.blockCount;
  
    if ((knownBlockCount - blockCount < THRESHOLD) && (blockCount > 1)) {
      document.getElementById("wallet_status").className = "green-input";
    } else {
      document.getElementById("wallet_status").className = "orange-input";
    }
    document.getElementById("available_balance").textContent = availableBalance.toFixed(2);
    document.getElementById("locked_amount").textContent = lockedAmount.toFixed(2);
    document.getElementById("block_count").textContent = blockCount + "/" + knownBlockCount;
    console.log("checking wallet...");
  }

function confirmation() {
    let dest = document.getElementById("send_to").value;
    let amount = document.getElementById("send_amount").value;
    let conf_msg = document.getElementById("send_confirmation");
    conf_msg.textContent = "You are sending "+amount+" TRTL to: "+ document.getElementById("send_to").value;
}
  window.setInterval(setWalletStatus, 5000);