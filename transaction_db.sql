-- setup transaction database
CREATE DATABASE tx_history;
\c tx_history;

CREATE TABLE addresses (
	id SERIAL NOT NULL PRIMARY KEY,
	address CHAR(99) NOT NULL UNIQUE,
	blockHeight INT DEFAULT 1
);

CREATE TABLE transactions (
	id SERIAL NOT NULL PRIMARY KEY,
	addr_id SERIAL REFERENCES addresses(id),
	amount NUMERIC(15,2) NOT NULL,
	hash CHAR(64) NOT NULL,
	_timestamp TIMESTAMP NOT NULL,
	paymentID CHAR(64) NOT NULL
);
