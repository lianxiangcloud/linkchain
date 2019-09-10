package bootnode

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

var (
	bootNodeLocker   sync.RWMutex
	MainnetBootnodes = []string{
		"https://39.97.128.184:443",
		"https://39.97.197.181:443",
		"https://120.55.156.239:443",
		"https://47.110.211.42:443",
		"https://47.91.221.28:443",
		"https://161.117.157.31:443",
	}
	index = rand.Intn(len(MainnetBootnodes))
)

//UpdateBootNode update MainnetBootnodes from bootnodeAddrs,bootnodeAddrs's format are like https://ip1:port1,https://ip2:port2
func UpdateBootNode(bootnodeAddrs string, logger log.Logger) {
	var bootNodes []string
	endpoints := strings.Split(bootnodeAddrs, ",")
	if len(endpoints) == 0 {
		logger.Info("len(endpoints) == 0")
		netinfo := strings.Split(bootnodeAddrs, ":")
		if len(netinfo) != 0 {
			logger.Info("len(netinfo) != 0")
			bootNodes = append(bootNodes, bootnodeAddrs)
		}
	} else {
		logger.Info("len(endpoints) != 0")
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
	}
	if len(bootNodes) > 0 {
		bootNodeLocker.Lock()
		index = rand.Intn(len(bootNodes))
		MainnetBootnodes = bootNodes
		bootNodeLocker.Unlock()
	}
	logger.Info("UpdateBootNode", "index", index, "len(endpoints)", len(endpoints), "len(bootNodes)", len(bootNodes))
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
