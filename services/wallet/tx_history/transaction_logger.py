#!/usr/bin/env python3
import os
import psycopg2
import time


host = os.getenv('DB_HOST')
if host == None:
    print('Set the DB_HOST environment variable')
    exit(1)
db_user = os.getenv('DB_USER')
if db_user == None:
    print('Set the DB_USER environment variable')
    exit(1)
db_pwd = os.getenv('DB_PWD')
if db_pwd == None:
    print('Set the DB_PWD environment variable')
    exit(1)
db_conf = 'host={} dbname={} user={} password={}'.format(host,
                                                         'tx_history',
                                                         db_user,
                                                         db_pwd)
conn = psycopg2.connect(db_conf)
db = conn.cursor()
print('Connected to the database.')


def add_transaction(address, dest, amount, tx_id, date):
    db.execute('INSERT INTO transactions (source, dest, amount, hash, date) \
                VALUES (%s, %s, %s, %s, %s);', (address, dest, amount, tx_id, date))
    conn.commit()


def main():
    with open('tx.log', 'r') as f:
      while True:
        line = f.readline()
        if 'New transaction received' in line:
            print('received')
            line = list(filter(None, line.split(' ')))
            line += list(filter(None, f.readline().split(' ')))
            add_transaction(line[20], None, line[19][:-1], line[9][:-1], line[0])
        elif 'created and send' in line:
            print('sent')
            line = list(filter(None, line.split(' ')))
            line += list(filter(None, f.readline().split(' ')))
            line += list(filter(None, f.readline().split(' ')))
            add_transaction(line[25], line[21], '-'+line[20][:-1], line[10][:-1], line[0])
            db.execute('SELECT address FROM addresses \
                               WHERE address = %s;', (line[21],))
            print('ok')
            if db.fetchone() is not None:
                add_transaction(line[21], None, line[20][:-1], line[10][:-1], line[0])
        time.sleep(0.01)

if __name__ == '__main__':
    main()
