package bootnode

import (
	"fmt"
	"strings"
	"sync"
)

var (
	index            int
	bootNodeLocker   sync.RWMutex
	MainnetBootnodes = []string{
		"https://127.0.0.1:8087",
		"https://192.168.10.125:8087",
	}
)

//UpdateBootNode update MainnetBootnodes from bootnodeAddrs,bootnodeAddrs's format are like https://ip1:port1,https://ip2:port2
func UpdateBootNode(bootnodeAddrs string) {
	var bootNodes []string
	endpoints := strings.Split(bootnodeAddrs, ",")
	for i := 0; i < len(endpoints); i++ {
		var addr string
		netinfo := strings.Split(endpoints[i], ":")
		if len(netinfo) == 2 { //maybe is ip:port, not https://ip1:port1
			addr = fmt.Sprintf("https://%s", endpoints[i])
		} else {
			addr = endpoints[i]
		}
		bootNodes = append(bootNodes, addr)
	}
	if len(bootNodes) > 0 {
		bootNodeLocker.Lock()
		MainnetBootnodes = bootNodes
		bootNodeLocker.Unlock()
	}
}

func GetBootNodesNum() int {
	bootNodeLocker.RLock()
	num := len(MainnetBootnodes)
	bootNodeLocker.RUnlock()
	return num
}

func GetBestBootNode() (bootNodeAddr string) {
	bootNodeLocker.RLock()
	if len(MainnetBootnodes) != 0 {
		index = index % len(MainnetBootnodes)
		bootNodeAddr = MainnetBootnodes[index]
		index++
	}
	bootNodeLocker.RUnlock()
	return
}
