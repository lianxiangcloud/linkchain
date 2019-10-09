package p2p

import (
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/bootnode"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/types"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

const (
	dialOutInterval   = (20 * time.Second)
	dhtLookUpInterval = (5 * time.Minute)
)

//ConManager is the con manager of P2P
type ConManager struct {
	sw                   *Switch
	lookupBuf            []*common.Node // current discovery lookup results
	randomNodesFromCache []*common.Node // filled from Table
	lookupChan           chan int
	stopChan             chan bool
	candidateChan        chan []*types.CandidateState
	logger               log.Logger
	closeOnce            sync.Once
}

//NewConManager return the ConManager
func NewConManager(sw *Switch, log log.Logger) *ConManager {
	if sw.ntab == nil {
		log.Info("NewConManager tb is nil")
		return nil
	}
	manager := &ConManager{
		sw:                   sw,
		randomNodesFromCache: make([]*common.Node, sw.ntab.GetMaxConNumFromCache()),
		lookupChan:           make(chan int),
		stopChan:             make(chan bool),
		candidateChan:        make(chan []*types.CandidateState, 2),
		logger:               log,
	}

	return manager
}

//Start start the service of dialout
func (conma *ConManager) Start() {
	conma.logger.Info("ConManager Start")
	go conma.dialOutLoop()
	go conma.dialNodesFromNetLoop()
	go conma.typeChangeProbeLoop()
}

//Stop stop the service of ConManager
func (conma *ConManager) Stop() {
	conma.closeOnce.Do(func() {
		if conma.stopChan != nil {
			close(conma.stopChan)
		}
		if conma.candidateChan != nil {
			close(conma.candidateChan)
		}
	})
}

func (conma *ConManager) dialOutLoop() {
	conma.logger.Info("ConManager dialOutLoop")
	var (
		out, dialing, needDynDials int
		maxDialOutNums             int
	)
	d := time.Duration(dialOutInterval)
	timer := time.NewTimer(0)
	var needLookUp = false
	defer timer.Stop()
	for {
		maxDialOutNums = conma.sw.ntab.GetMaxDialOutNum() //The total number of maximum outward active connections
		select {
		case <-conma.stopChan:
			return
		case <-timer.C:
			out, _, dialing = conma.sw.NumPeers()
			needDynDials = maxDialOutNums - (out + dialing)
			conma.logger.Debug("dialOutLoop", "maxDialOutNums", maxDialOutNums, "needDynDials", needDynDials)
			if needDynDials > 0 {
				needDynDials = conma.dialRandNodesFromCache(needDynDials)
			}
			conma.logger.Debug("after dialRandNodesFromCache", "needDynDials", needDynDials)
			if needDynDials > 0 {
				if bootnode.GetLocalNodeType() == types.NodePeer && !needLookUp {
					needLookUp = true
					//for peer node,call dialRandNodesFromNet will send udp packet,so we should ensure that intervals are not too frequent
					timer.Reset(time.Duration(dhtLookUpInterval))
					continue
				} else {
					needLookUp = false
					conma.dialRandNodesFromNet(needDynDials)
				}
			}
			timer.Reset(d)
		}
	}
}

func (conma *ConManager) dialRandNodesFromCache(needDynDials int) int {
	n := conma.sw.ntab.ReadRandomNodes(conma.randomNodesFromCache)
	isDialingMap := make(map[string]bool)
	var nodeid string
	for i := 0; i < n && i < len(conma.randomNodesFromCache) && needDynDials > 0; i++ {
		nodeid = conma.randomNodesFromCache[i].ID.String()
		conma.logger.Debug("dialRandNodesFromCache", "i", i, "nodeid", nodeid, "IP",
			conma.randomNodesFromCache[i].IP.String(), "tcpPort", conma.randomNodesFromCache[i].TCP_Port)
		_, ok := isDialingMap[nodeid]
		if ok {
			continue
		}
		if conma.sw.AddDial(conma.randomNodesFromCache[i]) {
			isDialingMap[nodeid] = true
			needDynDials--
		}
	}
	return needDynDials
}

func (conma *ConManager) dialRandNodesFromNet(needDynDials int) {
	select {
	case conma.lookupChan <- needDynDials:
		return
	default:
		conma.logger.Debug("lookupChan block")
		return
	}
}

func (conma *ConManager) dialNodesFromNetLoop() {
	var needDynDials int
	for {
		select {
		case needDynDials = <-conma.lookupChan:
			conma.logger.Debug("dialNodesFromNetLoop", "needDynDials", needDynDials)
			lookupNodes := conma.sw.ntab.LookupRandom()
			isDialingMap := make(map[string]bool)
			var nodeid string
			for i := 0; i < len(lookupNodes) && needDynDials > 0; i++ {
				nodeid = lookupNodes[i].ID.String()
				_, ok := isDialingMap[nodeid]
				if ok {
					continue
				}
				if conma.sw.AddDial(lookupNodes[i]) {
					isDialingMap[nodeid] = true
					needDynDials--
				}
			}
		case <-conma.stopChan:
			return
		}
	}
}

func (conma *ConManager) typeChangeProbeLoop() {
	for {
		select {
		case <-conma.stopChan:
			return
		case candidates, _ := <-conma.candidateChan:
			if candidates != nil {
				conma.tryToSwitchNetWork(candidates)
			}
		}
	}
}

func (conma *ConManager) tryToSwitchNetWork(candidates []*types.CandidateState) {
	myType := bootnode.GetLocalNodeType()
	conma.logger.Info("tryToSwitchNetWork", "candidates", candidates, "myoldType", myType)
	typeChangeFlag := false
	findPukey := false
	for i := 0; i < len(candidates); i++ {
		if candidates[i] != nil {
			if string(candidates[i].PubKey.Bytes()) == string(conma.sw.nodeKey.PubKey().Bytes()) {
				findPukey = true
				break
			}
		}
	}

	if findPukey {
		if myType == types.NodePeer {
			typeChangeFlag = true
		}
	} else {
		if myType == types.NodeValidator {
			typeChangeFlag = true
		}
	}

	//get seeds from bootNode
	if typeChangeFlag {
		maxTryNum := 30
		for i := 0; i < maxTryNum; i++ {
			var bootNodeAddr = bootnode.GetBestBootNode()
			conma.logger.Info("typeChange start", "old myType", myType, "bootnodeAddr", bootNodeAddr)
			seeds, getType, err := bootnode.GetSeeds(bootNodeAddr, conma.sw.nodeKey, conma.logger)
			if err != nil {
				continue
			}
			if myType == getType { //bootnode refresh delay
				time.Sleep(time.Second)
			} else {
				conma.logger.Debug("ntab.Stop()", "myType", myType)
				conma.sw.ntab.Stop()
				needDht := false
				if getType == types.NodePeer {
					needDht = true
				}
				err := conma.sw.DefaultNewTable(seeds, needDht, true) //udp con have closed by table.Stop,so we should renew udpCon
				if err != nil {
					conma.logger.Info("DefaultNewTable", "sw.ntab err", err)
					return
				}
				conma.sw.ntab.Start()
				go conma.waitConToNewSeeds(seeds, getType)
				return
			}
		}
		conma.logger.Report("typeChange failed,bootnode not refreshed", "logID", types.LogIdBootNodeFail, "myType", myType)
	}
}

func (conma *ConManager) waitConToNewSeeds(newSeeds []*common.Node, getType types.NodeType) {
	conma.logger.Info("waitConToNewSeeds", "len(newSeeds)", len(newSeeds))
	privOutNum, _, _ := conma.sw.NumPeers()
	peers := conma.sw.Peers().List()
	maxWaitNum := 60
	netWorkChangeFlag := false
	isDialingMap := make(map[string]bool)
	//connnect to new seeds
	for i := 0; i < len(newSeeds); i++ {
		conma.logger.Info("start connect to new seeds", "i", i, "IP", newSeeds[i].IP.String(),
			"TCP_PORT", newSeeds[i].TCP_Port, "UDP_PORT", newSeeds[i].UDP_Port, "ID", newSeeds[i].ID.String())
		nodeid := newSeeds[i].ID.String()
		_, ok := isDialingMap[nodeid]
		if ok {
			continue
		}
		if conma.sw.AddDial(newSeeds[i]) {
			isDialingMap[nodeid] = true
		}
	}

	for i := 0; i < maxWaitNum; i++ {
		currentOutNum, _, _ := conma.sw.NumPeers()
		if currentOutNum > privOutNum { //connect to new seeds success,we should close the connection with old node
			netWorkChangeFlag = true
			break
		}
		time.Sleep(time.Second)
	}
	if netWorkChangeFlag {
		for _, peer := range peers {
			conma.sw.StopPeerForError(peer, "network change,Close old Connection")
		}
		conma.logger.Info("typeChange success", "getType", getType)
		return
	}
	conma.logger.Info("typeChange failed,still use old network")
}

//SetCandidate is called when candidate ndoe changed
func (conma *ConManager) SetCandidate(candidates []*types.CandidateState) {
	conma.logger.Info("SetCandidate", "candidates", candidates, "len(candidates)", len(candidates))
	myType := bootnode.GetLocalNodeType()
	if myType != types.NodePeer && myType != types.NodeValidator {
		conma.logger.Info("myType is not peer or validator,do not change network", "myType", myType)
		return
	}
	select {
	case _, ok := <-conma.stopChan:
		if !ok {
			conma.logger.Info("SetCandidate failed,conma already stop")
		}
	default:
		break
	}
	tmpCandis := make([]*types.CandidateState, len(candidates))
	for i := 0; i < len(tmpCandis); i++ {
		tmpCandis[i] = candidates[i]
	}
	select {
	case conma.candidateChan <- tmpCandis:
		return
	default:
		conma.logger.Info("candidateChan is full")
		break
	}
}
