# Shellnet

![screenshot](/docs/screenshot-shellnet-login.png)

## Setup on Ubuntu 16.04+
Install the required packages.  
`sudo apt install git postgresql postgresql-contrib redis-server`  
[Install golang-1.10](https://gist.github.com/ndaidong/4c0e9fbae8d3729510b1c04eb42d2a80)

Don't forget to make your GOPATH export persistent.

Install the necessary go libraries
```
go get github.com/gomodule/redigo/redis \
	github.com/julienschmidt/httprouter \
	github.com/lib/pq \
	github.com/opencoff/go-srp
```

Clone the Shellnet repo in your ${GOPATH}/src.

#### Postgres Setup
[Configure postgres user](https://www.linode.com/docs/databases/postgresql/how-to-install-postgresql-on-ubuntu-16-04/)  
Setup user database  
`~$ cat user_db.sql | psql -U <username> -h <host>`  
Setup transactions database  
`~$ cat transaction_db.sql | psql -U <username> -h <host>`

#### Setup Turtlecoin service
Run this once.
`~$ ./turtle-service --container-file arg -p <password> -g`  

Start turtle-service
`~$ ./turtle-service --rpc-password <password> --container-file arg -p <password> -d`
Point turtle-service at an existing daemon like this
`~$ ./turtle-service --rpc-password <password> --container-file arg -p <password> -d --daemon-address <daemon DNS or IP address> --daemon-port <daemon port>`

#### Start redis-server

#### Configure and start run scripts
Edit these files:
* services/main/run.sh  
```bash
#!/usr/bin/env bash
HOST_URI=<Web wallet DNS or IP address> \
HOST_PORT=<Web wallet port> \
USER_URI='http://localhost:8081' \
WALLET_URI='http://localhost:8082' \
go run main.go utils.go
```
* services/wallet/run.sh  
```bash
#!/usr/bin/env bash
DB_USER=<postgres username> \
DB_PWD=<postgres password> \
HOST_URI='http://localhost' \
HOST_PORT=':8082' \
RPC_PWD=<turtle-service RPC password>  \
RPC_PORT=':8070' \
go run wallet.go utils.go
```
* services/user/run.sh  
```bash
#!/usr/bin/env bash
DB_USER=<postgres username> \
DB_PWD=<postgres password> \
HOST_URI='http://localhost' \
HOST_PORT=':8081' \
WALLET_URI='http://localhost:8082' go run users.go utils.go
```

`~$ cd services/main ; ./run.sh & disown`  
`~$ cd services/wallet ; ./run.sh & disown`  
`~$ cd services/user ; ./run.sh & disown`  

## Todo
* Finish walletd integration
* Make Front-end pretty
* add documentation
* automate tasks
* add tests


## Dependencies
* Redis
* Postgresql
* Go
* TurtleCoin wallet daemon
