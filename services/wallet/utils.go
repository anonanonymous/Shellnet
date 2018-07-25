package main

import "errors"

// wrapper for redis SET KEY VAL, expire after 420 hours
func sessionSetKey(key, val string) error {
	_, err := sessionDB.Do("SET", key, val, "EX", 1512000)
	if err != nil {
		return err
	}
	return nil
}

// retrieve username from sessionid
func sessionGetKey(key string) (string, error) {
	reply, err := sessionDB.Do("GET", key)
	if err != nil {
		return "", err
	}
	if reply == nil {
		return "", errors.New("Key not found")
	}
	username := string(reply.([]byte))
	return username, nil
}
