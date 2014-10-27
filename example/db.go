package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	log "github.com/kdar/factorlog"
)

// The key-value database.
type DB struct {
	data  map[string]string
	mutex sync.RWMutex
}

// Creates a new database.
func dbNew() *DB {
	return &DB{
		data: make(map[string]string),
	}
}

func (db *DB) Read(key []string) ([]string, error) {
	if len(key) != 1 {
		fmt.Println("read needs 1 key:", key)
		return nil, errors.New("Read needs 1 key")
	}
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	log.Println("GET ", key[0], " ", db.data[key[0]])
	return []string{db.data[key[0]]}, nil
}

func (db *DB) Write(key []string) ([]string, error) {
	if len(key) != 2 {
		fmt.Println("write needs 2 keys:", key)
		return nil, errors.New("Write needs 2 keys:")
	}
	log.Println("PUT ", key[0], " ", key[1])
	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.data[key[0]] = key[1]
	return []string{"ok"}, nil
}

func (db *DB) Save() ([]byte, error) {
	b, err := json.Marshal(db.data)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (db *DB) Recovery(in []byte) error {
	return json.Unmarshal(in, &db.data)
}
