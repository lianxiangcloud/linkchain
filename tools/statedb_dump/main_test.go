package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/state"
)

func TestJSONDump(t *testing.T) {
	isTrie := false
	typeStr := "kv"
	if isTrie {
		typeStr = "tr"
	}
	fname := fmt.Sprintf("acc_%s_%s.log", typeStr, time.Now().Format("20060102-15-04-05"))
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var (
		stateRoot  common.Hash
		dbpath     string
		stateStore dbm.DB
		stateDB    *state.StateDB
	)
	stateRoot = common.HexToHash("0x44648cebef840906a0e09686f96263c23c6d84659db702b203fdbc967d0b267b")
	dbpath = "./dump/db/kv2/"
	if isTrie {
		stateRoot = common.HexToHash("0x44648cebef840906a0e09686f96263c23c6d84659db702b203fdbc967d0b267b")
		dbpath = "./dump/db/full/"
	}

	stateStore = dbm.NewDB("state", "leveldb", dbpath, 0)
	defer stateStore.Close()
	stateDB, err = state.New(stateRoot, state.NewKeyValueDBWithCache(stateStore, 0, isTrie, 0))
	if err != nil {
		panic(err)
	}

	fmt.Println("start dump", typeStr, time.Now())
	accs := stateDB.JSONDumpKV()
	fmt.Println("start dump", typeStr, time.Now())
	if len(accs) == 0 {
		return
	}
	for _, str := range accs {
		if _, err := f.WriteString(str); err != nil {
			panic(err)
		}
		f.WriteString("\n")
	}
}
