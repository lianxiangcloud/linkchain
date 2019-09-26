package sync

import (
	"time"

	"github.com/lianxiangcloud/linkchain/app"
	"github.com/lianxiangcloud/linkchain/bootnode"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/types"
)

const (
	maxSameHeightCount               = 2
	minCheckInterval   time.Duration = time.Second * 10
)

//SyncHeightManager is the struct to check wether the height of our node is the same with the height of linkchain
type SyncHeightManager struct {
	cmn.BaseService
	sw            *p2p.Switch
	app           *app.LinkApplication
	logger        log.Logger
	checkInterval time.Duration
	stopChan      chan bool
}

func NewSyncHeightManager(sw *p2p.Switch, app *app.LinkApplication, checkInterval time.Duration, logger log.Logger) *SyncHeightManager {
	if sw == nil || app == nil {
		return nil
	}
	logger.Info("NewSyncHeightManager", "checkInterval", checkInterval)
	if checkInterval < minCheckInterval {
		logger.Info("checkInterval < minCheckInterval", "checkInterval", checkInterval, "minCheckInterval", minCheckInterval)
		checkInterval = minCheckInterval
	}
	shm := &SyncHeightManager{
		sw:            sw,
		app:           app,
		logger:        logger,
		checkInterval: checkInterval,
		stopChan:      make(chan bool),
	}
	shm.BaseService = *cmn.NewBaseService(logger, "SyncHeightManager", shm)
	return shm
}

func (sm *SyncHeightManager) OnStart() error {
	sm.Logger.Debug("SyncHeightManager OnStart")
	go sm.heightProbe()
	return nil
}

func (sm *SyncHeightManager) OnStop() {
	close(sm.stopChan)
}

func (sm *SyncHeightManager) heightProbe() {
	var myCurrentHeight uint64
	var sameHeightCount int
	//check := time.NewTicker(sm.checkInterval)
	//defer check.Stop()
	timer := time.NewTimer(sm.checkInterval)
	defer timer.Stop()
	myLastHeight := sm.app.Height()
	for {
		select {
		case <-sm.stopChan:
			return
		case <-timer.C:
			if sameHeightCount >= maxSameHeightCount {
				myCurrentHeight = sm.app.Height()
				if myCurrentHeight > myLastHeight { //maybe the block is syning,but it is just very slowly
					sameHeightCount = 0
					myLastHeight = myCurrentHeight
					timer.Reset(sm.checkInterval)
					continue
				}
				myLastHeight = myCurrentHeight

				lkchainHeight, err := bootnode.GetCurrentHeightOfChain(sm.logger)
				if err != nil {
					sm.logger.Report("SyncHeightManager", "logID", types.LogIdBootNodeFail, "getCurrentHeightOfChain err", err, "bootnodeAddr", sm.sw.BootNodeAddr())
					timer.Reset(minCheckInterval)
					continue
				}
				if lkchainHeight >= (myCurrentHeight + uint64(maxSameHeightCount)) { //we should 	get the seed node agian and change the nodes we have connected
					if bootnode.GetLocalNodeType() != types.NodePeer && bootnode.GetLocalNodeType() != types.NodeValidator &&
						bootnode.GetLocalNodeType() != 0 {
						sm.logger.Report("SyncHeightManager", "logID", types.LogIdSyncBlockFail, "type", bootnode.GetLocalNodeType(), "height", sm.app.Height())
						return
					}
					seeds, getType, err := bootnode.GetSeeds(sm.sw.BootNodeAddr(), sm.sw.NodeKey(), sm.logger)
					if err != nil {
						sm.logger.Report("SyncHeightManager", "logID", types.LogIdBootNodeFail, "GetSeeds err", err, "bootnodeAddr", sm.sw.BootNodeAddr())
						timer.Reset(minCheckInterval)
						continue
					}
					sm.logger.Info("SyncHeightManager heightProbe get the seeds again", "lkchainHeight", lkchainHeight, "myCurrentHeight", myCurrentHeight, "getType", getType)
					for i := 0; i < len(seeds); i++ {
						sm.logger.Info("GetSeedsFromBootSvr", " seeds i", i, "ip", seeds[i].IP.String(), "UDP_Port", seeds[i].UDP_Port, "TCP_Port", seeds[i].TCP_Port)
					}
					sm.sw.GetTable().Stop()
					//try to connect to new seeds and renew dht table
					sm.connectToNewSeeds(seeds)
					needDht := false
					if getType == types.NodePeer {
						needDht = true
					}
					err = sm.sw.DefaultNewTable(seeds, needDht, true) //udp con have closed by table.Stop,so we should renew udpCon
					if err != nil {
						sm.logger.Info("DefaultNewTable", "sw.ntab err", err)
						return
					}
					sm.sw.GetTable().Start()
					timer.Reset(minCheckInterval)
				} else {
					sameHeightCount = 0
					timer.Reset(sm.checkInterval)
					continue
				}
			} else {
				myCurrentHeight = sm.app.Height()
				if myCurrentHeight == myLastHeight {
					sameHeightCount++
				} else {
					sameHeightCount = 0
				}
				myLastHeight = myCurrentHeight
				timer.Reset(sm.checkInterval)
			}
		}
	}
}

func (sm *SyncHeightManager) connectToNewSeeds(newSeeds []*common.Node) {
	sm.logger.Info("connectToNewSeeds", "len(newSeeds)", len(newSeeds))
	peers := sm.sw.Peers().List()
	isDialingMap := make(map[string]bool)
	//connnect to new seeds
	for i := 0; i < len(newSeeds); i++ {
		sm.logger.Info("start connect to new seeds", "i", i, "IP", newSeeds[i].IP.String(),
			"TCP_PORT", newSeeds[i].TCP_Port, "UDP_PORT", newSeeds[i].UDP_Port, "ID", newSeeds[i].ID.String())
		nodeid := newSeeds[i].ID.String()
		_, ok := isDialingMap[nodeid]
		if ok {
			continue
		}
		if sm.sw.AddDial(newSeeds[i]) {
			isDialingMap[nodeid] = true
		}
	}

	newPeers := sm.sw.Peers().List()
	var num = 0
	if len(peers) == len(newPeers) {
	Loop:
		for i := 0; i < len(peers); i++ {
			for j := 0; j < len(newPeers); j++ {
				if peers[i].ID() == newPeers[j].ID() {
					num++
					continue Loop
				}
			}
		}
		if len(peers) == num {
			sm.logger.Info("old seeds are the same with the new seeds,keep the connection with old seeds", "num", num)
			return
		}
	}

	for _, peer := range peers {
		sm.sw.StopPeerForError(peer, "network change,Close old Connection")
	}
	sm.logger.Info("connectToNewSeeds done")
}
