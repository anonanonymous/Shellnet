-- setup user database
CREATE database users;
\c users;
CREATE TABLE accounts (
IH char(64) NOT NULL,
Verifier char(585) NOT NULL,
Username varchar(64) NOT NULL UNIQUE,
ID  SERIAL PRIMARY KEY,
address char(99) NOT NULL);