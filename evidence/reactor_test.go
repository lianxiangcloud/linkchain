package evidence

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/blockchain"
	cfg "github.com/lianxiangcloud/linkchain/config"
	cs "github.com/lianxiangcloud/linkchain/consensus"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/mempool"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/version"
	"github.com/stretchr/testify/assert"
)

// connect N evidence reactors through N switches

func generateVals(valsNum int, startPort int) ([]crypto.PrivKey, []*common.Node) {
	privKeys := make([]crypto.PrivKey, valsNum)
	validators := make([]*common.Node, valsNum)

	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = crypto.GenPrivKeyEd25519()
		validators[i] = &common.Node{IP: net.ParseIP("127.0.0.1"), TCP_Port: uint16(startPort + i), ID: common.NodeID(crypto.Keccak256Hash(privKeys[i].PubKey().Bytes()))}
	}
	return privKeys, validators
}

func makeNodeInfo(chainID string, nodeType types.NodeType, moniker string, httpEndpoint string) p2p.NodeInfo {
	nodeInfo := p2p.NodeInfo{
		Network: chainID,
		Version: version.Version,
		Channels: []byte{
			blockchain.BlockchainChannel,
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mempool.MempoolChannel,
			EvidenceChannel,
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

func makeAndConnectEvidenceReactors(stateDBs []dbm.DB, statess []cs.NewStatus, startPort int) []*EvidenceReactor {
	N := len(stateDBs)
	reactors := make([]*EvidenceReactor, N)
	logger := log.Root()
	privKeys, seeds := generateVals(N, startPort)
	for i := 0; i < N; i++ {
		store := NewEvidenceStore(dbm.NewMemDB())
		pool := NewEvidencePool(stateDBs[i], store, statess[i])
		reactors[i] = NewEvidenceReactor(pool)
		reactors[i].SetLogger(logger.With("validator", i))
		config := cfg.DefaultP2PConfig()
		config.ListenAddress = fmt.Sprintf(":%d", seeds[i].TCP_Port)

		localNodeInfo := makeNodeInfo("chainID", types.NodeValidator, fmt.Sprintf("validator%d", i), "")

		p2pmanager, err := p2p.NewP2pManager(logger, "", privKeys[i], config, localNodeInfo, seeds, stateDBs[i])
		if err != nil {
			panic(fmt.Sprintf("NewP2pManager err %v", err))
		}
		reactors[i].SetP2PManager(p2pmanager)
		p2pmanager.AddReactor("EVIDENCE", reactors[i])
		p2pmanager.Start()
	}

	return reactors
}

// wait for all evidence on all reactors
func waitForEvidence(t *testing.T, evs types.EvidenceList, reactors []*EvidenceReactor) {
	// wait for the evidence in all evpools
	wg := new(sync.WaitGroup)
	for i := 0; i < len(reactors); i++ {
		wg.Add(1)
		go _waitForEvidence(t, wg, evs, i, reactors)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(TIMEOUT)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for evidence")
	case <-done:
	}
}

// wait for all evidence on a single evpool
func _waitForEvidence(t *testing.T, wg *sync.WaitGroup, evs types.EvidenceList, reactorIdx int, reactors []*EvidenceReactor) {

	evpool := reactors[reactorIdx].evpool
	for len(evpool.PendingEvidence()) != len(evs) {
		time.Sleep(time.Millisecond * 100)
	}
	reapedEv := evpool.PendingEvidence()
	// put the reaped evidence in a map so we can quickly check we got everything
	evMap := make(map[string]types.Evidence)
	for _, e := range reapedEv {
		evMap[string(e.Hash())] = e
	}
	for i, expectedEv := range evs {
		gotEv := evMap[string(expectedEv.Hash())]
		assert.Equal(t, expectedEv, gotEv,
			fmt.Sprintf("evidence at index %d on reactor %d don't match: %v vs %v",
				i, reactorIdx, expectedEv, gotEv))
	}

	wg.Done()
}

func sendEvidence(t *testing.T, evpool *EvidencePool, valAddr []byte, n int) types.EvidenceList {
	evList := make([]types.Evidence, n)
	for i := 0; i < n; i++ {
		ev := types.NewMockGoodEvidence(uint64(i+1), 0, valAddr)
		err := evpool.AddEvidence(ev)
		assert.Nil(t, err)
		evList[i] = ev
	}
	return evList
}

var (
	NUM_EVIDENCE = 5
	TIMEOUT      = 120 * time.Second // ridiculously high because CircleCI is slow
)

func TestReactorBroadcastEvidence(t *testing.T) {
	N := 3
	// create statedb for everyone
	stateDBs := make([]dbm.DB, N)
	statess := make([]cs.NewStatus, N)
	valAddr := []byte("myval")
	// we need validators saved for heights at least as high as we have evidence for
	height := uint64(NUM_EVIDENCE) + 10
	for i := 0; i < N; i++ {
		stateDBs[i], statess[i] = initializeValidatorState(valAddr, height)
	}

	// make reactors from statedb
	reactors := makeAndConnectEvidenceReactors(stateDBs, statess, 13500)
	time.Sleep(5 * time.Second)
	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.sw.Peers().List() {
			ps := peerState{height}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// send a bunch of valid evidence to the first reactor's evpool
	// and wait for them all to be received in the others
	evList := sendEvidence(t, reactors[0].evpool, valAddr, NUM_EVIDENCE)
	waitForEvidence(t, evList, reactors)
}

type peerState struct {
	height uint64
}

func (ps peerState) GetHeight() uint64 {
	return ps.height
}

func TestReactorSelectiveBroadcast(t *testing.T) {
	valAddr := []byte("myval")
	height1 := uint64(NUM_EVIDENCE) + 10
	height2 := uint64(NUM_EVIDENCE) / 2

	// DB1 is ahead of DB2
	stateDB1, states1 := initializeValidatorState(valAddr, height1)
	stateDB2, states2 := initializeValidatorState(valAddr, height2)

	// make reactors from statedb
	reactors := makeAndConnectEvidenceReactors([]dbm.DB{stateDB1, stateDB2}, []cs.NewStatus{states1, states2}, 13600)

	time.Sleep(5 * time.Second)

	peer := reactors[0].sw.Peers().List()[0]
	ps := peerState{height2}
	peer.Set(types.PeerStateKey, ps)

	// send a bunch of valid evidence to the first reactor's evpool
	evList := sendEvidence(t, reactors[0].evpool, valAddr, NUM_EVIDENCE)

	// only ones less than the peers height should make it through
	waitForEvidence(t, evList[:NUM_EVIDENCE/2], reactors[1:2])

	// peers should still be connected
	peers := reactors[1].sw.Peers().List()
	assert.Equal(t, 1, len(peers))
}
