# Forking Shellnet

Forking should be easy for most recent TurtleCoin forks that work with turtle-service.

You DO NOT need to change any references to `turtle-service`.  Since `turtle-service` is using RPC, Shellnet doesn't care what what your forked service is called.

### Coin Settings
*services/wallet/wallet.go*
```go
var addressFormat = "^TRTL([a-zA-Z0-9]{95}|[a-zA-Z0-9]{183})$"
var divisor float64 = 100
var transactionFee = 10
```

services/main/assets/js/account.js
```js
const tickerSymbol = 'TRTL';
const decimalPlaces = 2;
```

In both database scripts in project root you may need to change address length to match yours.  
`address char(99)`

### Branding

Replace services/main/assets/images/brand-logo.png with your own logo.
Replace services/main/assets/images/background.jpeg with your own website background.

There are a few places you may need to make frontend changes for now  Here are some of them, just do a search for `TRTL` or `Turtle` to find any I missed.

#### services/main/templates/index.html
```html
<span class="tagline">A secure, easy-to-use wallet for TurtleCoin payments</span>
```

#### services/main/templates/account.html
In `printf "%.2f"`, 2f is the number of decimal places to display. To show 4 decimal places, you'd use `printf "%.4f"`.  JS replaces the content of both spans on first wallet update.  
```html
<tr>
  <th>Name</th>
  <td>{{ .User.Username }}</td>
</tr>
<tr>
  <th>Available</th>
  <td><span id="available_balance">{{ printf "%.2f" (index .Wallet "balance" "availableBalance") }} TRTL</span></td>
</tr>
<tr>
  <th>Locked / Unconfirmed</th>
  <td><span id="locked_amount">{{ printf "%.2f" (index .Wallet "balance" "lockedAmount") }} TRTL</span></td>
</tr>
...
```
```html
<div class="table-container">
    <form action={{ printf "%s%s" .PageAttr.URI "/account/send_transaction"}} method="POST">
        <div class="input-field grey-input">
            <h2>Send Transaction</h2><small>fee: 0.1 TRTL</small><br>
            <span class="caret-icon"></span>
            <input id="send_to" type="text" name="destination" placeholder="Enter destination address..." pattern="^TRTL([a-zA-Z0-9]{95}|[a-zA-Z0-9]{183})\s*$" required/>
            <span class="amount-icon"></span>
            <input id="send_amount" type="text" name="amount" placeholder="Enter Amount.." pattern="^\d+\.{0,1}\d{0,6}$" required/>
            <span class="paymentid-icon"></span>
            <input type="text" name="payment_id" placeholder="Enter Payment ID..." pattern="^[a-fA-F\d]{64}$"/>
        </div>
...
```
```html
<div class="container tx">
 ...
<td><b>Amount</b><br>{{ index $ele "Amount" }}&nbsp;TRTL</td>
{{ else }}
<td><strong>Deposit</strong></td>
<td><b>Hash</b><br>{{ index $ele "Hash" }}<br><b>PaymentId</b><br>"{{ index $ele "PaymentID"}}"</td>
<td><b>Amount</b><br>{{ index $ele "Amount" }}&nbsp;TRTL</td>
{{ end }}
...
</div>
```

The rest is CSS.
