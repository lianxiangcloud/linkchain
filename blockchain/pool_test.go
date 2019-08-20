package blockchain

import (
	"testing"
	"time"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"

	"github.com/lianxiangcloud/linkchain/types"
)

func init() {
	peerTimeout = 2 * time.Second
}

type testPeer struct {
	id     string
	height uint64
}

func makePeers(numPeers int, minHeight, maxHeight int64) map[string]testPeer {
	peers := make(map[string]testPeer, numPeers)
	for i := 0; i < numPeers; i++ {
		peerID := string(cmn.RandStr(12))
		height := uint64(minHeight) + uint64(cmn.RandInt63n(maxHeight-minHeight))
		peers[peerID] = testPeer{peerID, height}
	}
	return peers
}

func TestBasic(t *testing.T) {
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(uint64(start), requestsCh, errorsCh)
	pool.SetLogger(log.Test())

	err := pool.Start()
	if err != nil {
		t.Error(err)
	}

	defer pool.Stop()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()
	defer func() {
		for _, peer := range peers {
			pool.RemovePeer(peer.id)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	for {
		select {
		case err := <-errorsCh:
			t.Error(err)
		case request := <-requestsCh:
			t.Logf("Pulled new BlockRequest %v", request)
			if request.Height == 300 {
				return // Done!
			}
			// Request desired, pretend like we got the block immediately.
			go func() {
				block := &types.Block{Header: &types.Header{Height: request.Height}}
				pool.AddBlock(request.PeerID, block, 123)
				t.Logf("Added block from peer %v (height: %v)", request.PeerID, request.Height)
			}()
		}
	}
}

func TestTimeout(t *testing.T) {
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(uint64(start), requestsCh, errorsCh)
	pool.SetLogger(log.Test())
	err := pool.Start()
	if err != nil {
		t.Error(err)
	}
	defer pool.Stop()

	for _, peer := range peers {
		t.Logf("Peer %v", peer.id)
	}

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	counter := 0
	timedOut := map[string]struct{}{}
	for {
		select {
		case err := <-errorsCh:
			t.Log(err)
			// consider error to be always timeout here
			if _, ok := timedOut[err.peerID]; !ok {
				counter++
				if counter == len(peers) {
					return // Done!
				}
			}
		case request := <-requestsCh:
			t.Logf("Pulled new BlockRequest %+v", request)
		}
	}
}

func TestBlockPool(t *testing.T) {
	peers := make(map[string]testPeer, 10)
	peerID := ""
	for i := 0; i < 10; i++ {
		peerID = string(cmn.RandStr(12))
		peers[peerID] = testPeer{peerID, uint64(i)}
	}
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(uint64(0), requestsCh, errorsCh)
	pool.SetLogger(log.Test())
	err := pool.Start()
	if err != nil {
		t.Error(err)
	}
	defer pool.Stop()

	for _, peer := range peers {
		t.Logf("Peer %v", peer.id)
	}

	for _, peer := range peers {
		pool.SetPeerHeight(peer.id, peer.height)
	}
	if pool.MaxPeerHeight() != 9 {
		t.Fatal("max peer height is not 9")
	}
	if pool.MockCaughtUp(true) == true {
		t.Fatal("pool mock caughtup status failed")
	}
	setPoolFlag(pool)
}

func setPoolFlag(pool *BlockPool) {
	pool.MockCaughtUp(false)
	pool.NeverCaughtUp(false)
	pool.IsCaughtUp()
	pool.debug()
}