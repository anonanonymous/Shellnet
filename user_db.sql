-- setup user database
CREATE database users;
\c users;
CREATE TABLE accounts (
	ih CHAR(64) NOT NULL,
	verifier CHAR(585) NOT NULL,
	username VARCHAR(64) NOT NULL UNIQUE,
	id  SERIAL PRIMARY KEY,
	totp VARCHAR(160) DEFAULT '',
	address CHAR(99) NOT NULL
);
