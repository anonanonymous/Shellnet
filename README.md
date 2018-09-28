# Shellnet

## Setup on Ubuntu 16.04
Install the required packages.  
`sudo apt install git postgresql postgresql-contrib redis-server`  
[Install golang-1.10](https://gist.github.com/ndaidong/4c0e9fbae8d3729510b1c04eb42d2a80)

Install the necessary go libraries
```
go get github.com/gomodule/redigo/redis \
	github.com/julienschmidt/httprouter \
	github.com/lib/pq \
	github.com/opencoff/go-srp
```

#### Postgres Setup
[Configure postgres user](https://www.linode.com/docs/databases/postgresql/how-to-install-postgresql-on-ubuntu-16-04/)  
Setup user database  
`~$ cat user_db.sql | psql -U <username> -h <host>`  
Setup transactions database  
`~$ cat transaction_db.sql | psql -U <username> -h <host>`

#### Setup Turtlecoin service
`~$ ./turtle-service --container-file arg -p password -g`  
`~$ ./turtle-service --rpc-password password --container-file arg -p password -d`

#### Start redis-server

#### Configure and start run scripts
Edit
* services/main/run.sh  
* services/wallet/run.sh  
* services/user/run.sh  

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
