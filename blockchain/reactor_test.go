package blockchain

import (
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"fmt"

	cfg "github.com/lianxiangcloud/linkchain/config"
	cs "github.com/lianxiangcloud/linkchain/consensus"
	"github.com/lianxiangcloud/linkchain/evidence"
	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	pcmn "github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/mempool"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/version"
)

func initializeValidatorState(height uint64) (cs.NewStatus, *BlockStore) {
	stateDB := dbm.NewMemDB()

	// create validator set and state
	val, _ := types.RandValidator(false, 10)
	valSet := &types.ValidatorSet{}
	valSet.Validators = append(valSet.Validators, val)
	state := cs.NewStatus{
		LastBlockHeight:             0,
		LastBlockTime:               uint64(time.Now().Unix()),
		Validators:                  valSet,
		LastValidators:              valSet,
		LastHeightValidatorsChanged: 1,
	}
	state.ConsensusParams = *types.DefaultConsensusParams()

	// save all states up to height
	for i := uint64(0); i < height; i++ {
		state.LastBlockHeight = i
		cs.SaveStatus(stateDB, state)
	}

	blockDB := dbm.NewMemDB()
	blockStore := NewBlockStore(blockDB)

	return state, blockStore
}

func makeStateAndBlockStore() (cs.NewStatus, *BlockStore) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	// blockDB := dbm.NewDebugDB("blockDB", dbm.NewMemDB())
	// stateDB := dbm.NewDebugDB("stateDB", dbm.NewMemDB())
	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	blockStore := NewBlockStore(blockDB)
	state, err := cs.CreateStatusFromGenesisFile(stateDB, config.GenesisFile())
	if err != nil {
		panic(cmn.ErrorWrap(err, "error constructing state from genesis file"))
	}
	return state, blockStore
}

func newBlockchainReactor(logger log.Logger, maxBlockHeight uint64) *BlockchainReactor {
	state, blockStore := initializeValidatorState(0)
	// Make the blockchainReactor itself
	fastSync := true
	bcApp := &cs.MockBlockChainApp{}
	bcApp.On("Height").Return(state.LastBlockHeight)
	bcApp.On("CheckBlock", mock.Anything, mock.Anything).Return(true)
	bcApp.On("LoadBlock", mock.Anything).Return(func(height uint64) *types.Block {
		return blockStore.LoadBlock(height)
	})

	// bcApp.On("CheckProcessResult", mock.Anything, mock.Anything).Return(true)
	bcApp.On("CommitBlock", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, common.EmptyHash, nil)
	blockExec := cs.NewBlockExecutor(dbm.NewMemDB(), log.Test(), cs.MockEvidencePool{})
	bcReactor := NewBlockchainReactor(state.Copy(), blockExec, bcApp, fastSync, nil)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	// Next: we need to set a switch in order for peers to be added in
	// bcReactor.Switch = p2p.NewSwitch(cfg.DefaultP2PConfig())

	// Lastly: let's add some blocks in
	for blockHeight := uint64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		firstBlock := makeBlock(blockHeight)
		secondBlock := makeBlock(blockHeight + 1)
		firstParts := firstBlock.MakePartSet(state.ConsensusParams.BlockGossip.BlockPartSizeBytes)
		txsResult := &types.TxsResult{}
		blockStore.SaveBlock(firstBlock, firstParts, secondBlock.LastCommit, nil, txsResult)
	}
	bcReactor.RestartFastSync(state)

	return bcReactor
}

func generateVals(valsNum int, startPort int) ([]crypto.PrivKey, []*pcmn.Node) {
	privKeys := make([]crypto.PrivKey, valsNum)
	validators := make([]*pcmn.Node, valsNum)

	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = crypto.GenPrivKeyEd25519()
		validators[i] = &pcmn.Node{IP: net.ParseIP("127.0.0.1"), TCP_Port: uint16(startPort + i), ID: pcmn.NodeID(crypto.Keccak256Hash(privKeys[i].PubKey().Bytes()))}
	}
	return privKeys, validators
}

func makeNodeInfo(chainID string, nodeType types.NodeType, moniker string, httpEndpoint string) p2p.NodeInfo {
	nodeInfo := p2p.NodeInfo{
		Network: chainID,
		Version: version.Version,
		Channels: []byte{
			BlockchainChannel,
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mempool.MempoolChannel,
			evidence.EvidenceChannel,
		},
		Moniker: moniker,
		Other: []string{
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("consensus_version=%v", cs.Version),
		},
		Type: nodeType,
	}

	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", httpEndpoint))
	return nodeInfo
}

func TestNoBlockResponse(t *testing.T) {
	maxBlockHeight := uint64(20)

	bcr := newBlockchainReactor(log.Test(), maxBlockHeight)
	bcr.StopFastSync()
	bcr.Start()
	defer bcr.Stop()

	// Add some peers in
	peer := newbcrTestPeer(cmn.RandStr(12))
	bcr.AddPeer(peer)
	defer bcr.RemovePeer(peer, nil)

	chID := byte(0x01)

	tests := []struct {
		height   uint64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{100, false},
	}

	// receive a request message from peer,
	// wait for our response to be received on the peer
	for _, tt := range tests {
		reqBlockMsg := &bcBlockRequestMessage{tt.height}
		reqBlockBytes := ser.MustEncodeToBytesWithType(reqBlockMsg)
		bcr.Receive(chID, peer, reqBlockBytes)
		msg := peer.lastBlockchainMessage()

		if tt.existent {
			if blockMsg, ok := msg.(*bcBlockResponseMessage); !ok {
				t.Fatalf("Expected to receive a block response for height %d", tt.height)
			} else if blockMsg.Block.Height != tt.height {
				t.Fatalf("Expected response to be for height %d, got %d", tt.height, blockMsg.Block.Height)
			}
		} else {
			if noBlockMsg, ok := msg.(*bcNoBlockResponseMessage); !ok {
				t.Fatalf("Expected to receive a no block response for height %d", tt.height)
			} else if noBlockMsg.Height != tt.height {
				t.Fatalf("Expected response to be for height %d, got %d", tt.height, noBlockMsg.Height)
			}
		}
	}

	bcr.GetChannels()

	db := dbm.NewMemDB()
	logger := log.Root()
	privKeys, seeds := generateVals(1, 8000)
	config := cfg.DefaultP2PConfig()
	config.ListenAddress = fmt.Sprintf(":%d", seeds[0].TCP_Port)
	localNodeInfo := makeNodeInfo("chainID", types.NodeValidator, "validator", "")

	p2pmanager, err := p2p.NewP2pManager(logger, "", privKeys[0], config, localNodeInfo, seeds, db)
	if err != nil {
		panic(fmt.Sprintf("NewP2pManager err %v", err))
	}
	reactor := cs.NewConsensusReactor(nil, false, p2pmanager)
	bcr.sw = p2pmanager
	bcr.sw.AddReactor("CONSENSUS", reactor)
	for i := uint64(0); i < 10; i++ {
		block := &types.Block{Header: &types.Header{Height: i}}
		bcr.pool.AddBlock(peer.id, block, 123)
	}
	go bcr.poolRoutine()
	time.Sleep(5 * time.Second)
}

/*
// NOTE: This is too hard to test without
// an easy way to add test peer to switch
// or without significant refactoring of the module.
// Alternatively we could actually dial a TCP conn but
// that seems extreme.
func TestBadBlockStopsPeer(t *testing.T) {
	maxBlockHeight := uint64(20)

	bcr := newBlockchainReactor(log.Test(), maxBlockHeight)
	bcr.Start()
	defer bcr.Stop()

	// Add some peers in
	peer := newbcrTestPeer(p2p.ID(cmn.RandStr(12)))

	// XXX: This doesn't add the peer to anything,
	// so it's hard to check that it's later removed
	bcr.AddPeer(peer)
	assert.True(t, bcr.Switch.Peers().Size() > 0)

	// send a bad block from the peer
	// default blocks already dont have commits, so should fail
	block := bcr.store.LoadBlock(3)
	msg := &bcBlockResponseMessage{Block: block}
	peer.Send(BlockchainChannel, struct{ BlockchainMessage }{msg})

	ticker := time.NewTicker(time.Millisecond * 10)
	timer := time.NewTimer(time.Second * 2)
LOOP:
	for {
		select {
		case <-ticker.C:
			if bcr.Switch.Peers().Size() == 0 {
				break LOOP
			}
		case <-timer.C:
			t.Fatal("Timed out waiting to disconnect peer")
		}
	}
}
*/

//----------------------------------------------
// utility funcs

func makeTxs(height uint64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		nonce := uint64(i)
		tx := types.NewTransaction(nonce, common.HexToAddress("0x01"), big.NewInt(0), 100000, nil, nil)
		txs = append(txs, tx)
	}
	return
}

func makeBlock(height uint64) *types.Block {
	return types.MakeBlock(height, makeTxs(height), new(types.Commit))
}

// The Test peer
type bcrTestPeer struct {
	cmn.BaseService
	id string
	ch chan interface{}
}

var _ p2p.Peer = (*bcrTestPeer)(nil)

func newbcrTestPeer(peerID string) *bcrTestPeer {
	bcr := &bcrTestPeer{
		id: peerID,
		ch: make(chan interface{}, 2),
	}
	bcr.BaseService = *cmn.NewBaseService(nil, "bcrTestPeer", bcr)
	return bcr
}

func (tp *bcrTestPeer) lastBlockchainMessage() interface{} { return <-tp.ch }

func (tp *bcrTestPeer) TrySend(chID byte, msgBytes []byte) bool {
	var msg BlockchainMessage
	err := ser.DecodeBytes(msgBytes, &msg)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error while trying to parse a BlockchainMessage"))
	}
	if _, ok := msg.(*bcStatusResponseMessage); ok {
		// Discard status response messages since they skew our results
		// We only want to deal with:
		// + bcBlockResponseMessage
		// + bcNoBlockResponseMessage
	} else {
		tp.ch <- msg
	}
	return true
}

func (tp *bcrTestPeer) Send(chID byte, msgBytes []byte) bool { return tp.TrySend(chID, msgBytes) }
func (tp *bcrTestPeer) NodeInfo() p2p.NodeInfo               { return p2p.NodeInfo{} }
func (tp *bcrTestPeer) Status() p2p.ConnectionStatus         { return p2p.ConnectionStatus{} }
func (tp *bcrTestPeer) ID() string                           { return tp.id }
func (tp *bcrTestPeer) IsOutbound() bool                     { return false }
func (tp *bcrTestPeer) IsPersistent() bool                   { return true }
func (tp *bcrTestPeer) Get(s string) interface{}             { return s }
func (tp *bcrTestPeer) Set(string, interface{})              {}
func (tp *bcrTestPeer) RemoteIP() net.IP                     { return []byte{127, 0, 0, 1} }
func (tp *bcrTestPeer) OriginalAddr() *p2p.NetAddress        { return nil }
func (tp *bcrTestPeer) RemoteAddr() net.Addr                 { return nil }
func (tp *bcrTestPeer) Close() error                         { return nil }
