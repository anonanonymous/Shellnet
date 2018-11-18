# Forking Shellnet

Forking should be easy for most recent TurtleCoin forks that work with turtle-service.

### Coin Settings
services/wallet/wallet.go
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

In case I forgot anything, I put a comment `// Forking config` anywhere else I made changes.

### Branding

Replace services/main/assets/images/brand-logo.png with your own logo.
Replace services/main/assets/images/background.jpeg with your own website background.

The rest is CSS.
