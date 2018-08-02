# Shellnet

## Setup on Ubuntu 16.04
Install the required packages. 

`sudo apt install golang-1.9 git postgresql postgresql-contrib redis-server`
If the `go` command isn't available after installing golang, add the following to the end of your `~/.profile` and then `source . ~/.profile`
```
export GOPATH=$HOME/go
export GOROOT=/usr/lib/go-1.9
export PATH=$PATH:$GOROOT/bin
```
Install the necessary go libraries
```
go get github.com/gomodule/redigo/redis \
	github.com/julienschmidt/httprouter \
	github.com/lib/pq \
	github.com/microcosm-cc/bluemonday \
	github.com/opencoff/go-srp
```

#### Postgres Setup
Start the postgresql service
`sudo service postgresql start`

Temporarily switch to the postgres user and change the password of the `postgres` user
```
sudo su - postgres
postgres@hostname:~/ psql
postgres=# \password <your password>
```
To be continued...
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
