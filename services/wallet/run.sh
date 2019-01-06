#!/usr/bin/env bash
DB_USER= \
DB_PWD= \
HOST_URI='http://localhost' \
HOST_PORT=':8082' \
RPC_PWD=  \
RPC_PORT='8070' \
go run wallet.go utils.go
