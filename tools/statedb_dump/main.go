package main

import (
	"os"
	"fmt"
	"strconv"

	"github.com/lianxiangcloud/linkchain/libs/common"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage: ./statedb_dump <ethermint_db_path> <dump_path> <block_height> <is_kv>")
		return
	}
	ethermintDBPath := os.Args[1]
 	dumpPath        := os.Args[2]
 	blockHeightStr  := os.Args[3]
 	strIsKv         := os.Args[4]

 	isTrie := false
 	if strIsKv != "1" {
		isTrie = true
	}
	blockHeight, err := strconv.Atoi(blockHeightStr)
	if err != nil {
		fmt.Println("strconv atoi exec failed.", "err", err.Error(), "blockHeight", blockHeightStr)
		return
	}

	// create chaindata symlink chaindata.db
	err = createSymlink(ethermintDBPath)
	if err != nil {
		fmt.Println("createSymlink failed.")
		return
	}
	defer removeSymlink(ethermintDBPath)

	// get state root
	root, err := getStateRoot(ethermintDBPath, uint64(blockHeight))
	if err != nil {
		fmt.Println("get state root failed.", "err", err.Error(),
			"path", ethermintDBPath, "blockHeigh", blockHeight)
		return
	}

	allAccounts, err := getAccountList(ethermintDBPath, root)
	if err != nil {
		fmt.Println("get account list failed.", "state root", root.Hex())
		return
	}
	fmt.Println("get account list success.", "account_num", len(allAccounts))

	dumper := newStateDump(ethermintDBPath, root, dumpPath, isTrie)
	if dumper == nil {
		fmt.Println("newStateDump failed. dumper is nil")
		return
	}

	// start dump
	dumpStateRoot := common.EmptyHash.Hex()
	if isTrie {
		dumpStateRoot, err = dumper.dump(allAccounts)
		if err != nil {
			fmt.Println("dumper dump exec failed.", "err", err.Error())
			return
		}

		fmt.Println("begin to save root and height", "stateRoot", dumpStateRoot, "initBLockHeight", blockHeight+1)
		err = saveRootAndHeight(dumpPath, dumpStateRoot, strconv.Itoa(blockHeight+1))
		if err != nil {
			fmt.Println("save root and Height failed.", "err", err.Error())
			return
		}
		fmt.Println("statedb dump success.")
	} else {
		err = dumper.dumpKv(allAccounts)
		if err != nil {
			fmt.Println("dumper dumpKv exec failed.", "err", err.Error())
			return
		}
	}

	err = dumper.checkDump(allAccounts)
	if err != nil {
		fmt.Println("dumper check dump exec failed.", "err", err.Error())
		return
	}
	fmt.Println("check dump success.")

	fmt.Println("start to save msgs for check")
	err = dumper.saveDumpMsgsForCheck(common.HexToHash(dumpStateRoot), isTrie)
	if err != nil {
		fmt.Println("save dump msgs for check failed.", "err", err.Error())
		return
	}
	if !isTrie {
		err = dumper.saveFromMsgsForCheck(root)
		if err != nil {
			fmt.Println("save from msgs for check failed.", "err", err.Error())
			return
		}
	}

	fmt.Println("save msgs for check exec successful")
}
