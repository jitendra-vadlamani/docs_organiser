package storage

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v4"
)

type Store interface {
	Save(key string, value interface{}) error
	Load(key string, target interface{}) (bool, error)
	Close() error
}

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Suppress verbose logging

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) Save(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
}

func (s *BadgerStore) Load(key string, target interface{}) (bool, error) {
	var data []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get key from db: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return true, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return true, nil
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}
