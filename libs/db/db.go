package db

import "fmt"

//----------------------------------------
// Main entry

type DBBackendType   string
type DBBackendCounts uint64

const (
	LevelDBBackend   DBBackendType = "leveldb"   // legacy, defaults to goleveldb unless +gcc
	CLevelDBBackend  DBBackendType = "cleveldb"
	GoLevelDBBackend DBBackendType = "goleveldb"
	MemDBBackend     DBBackendType = "memdb"
	FSDBBackend      DBBackendType = "fsdb"      // using the filesystem naively
	BadgerBackend    DBBackendType = "badger"    // using badger
	BoltBackend      DBBackendType = "bolt"      // using bolt
)

type dbCreator func(name string, dir string, counts uint64) (DB, error)

var backends = map[DBBackendType]dbCreator{}

func registerDBCreator(backend DBBackendType, creator dbCreator, force bool) {
	_, ok := backends[backend]
	if !force && ok {
		return
	}
	backends[backend] = creator
}

func NewDB(name string, backend DBBackendType, dir string, counts uint64) DB {
	db, err := backends[backend](name, dir, counts)
	if err != nil {
		panic(fmt.Sprintf("Error initializing DB: %v", err))
	}
	return db
}
