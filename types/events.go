package types

import (
	"fmt"

	tmpubsub "github.com/lianxiangcloud/linkchain/libs/pubsub"
	tmquery "github.com/lianxiangcloud/linkchain/libs/pubsub/query"

	"github.com/lianxiangcloud/linkchain/libs/ser"
)

// Reserved event types
const (
	EventBond              = "Bond"
	EventCompleteProposal  = "CompleteProposal"
	EventDupeout           = "Dupeout"
	EventFork              = "Fork"
	EventLock              = "Lock"
	EventNewBlock          = "NewBlock"
	EventNewBlockHeader    = "NewBlockHeader"
	EventNewRound          = "NewRound"
	EventNewRoundStep      = "NewRoundStep"
	EventPolka             = "Polka"
	EventRebond            = "Rebond"
	EventRelock            = "Relock"
	EventTimeoutPropose    = "TimeoutPropose"
	EventTimeoutWait       = "TimeoutWait"
	EventTx                = "Tx"
	EventLog               = "Log"
	EventUnbond            = "Unbond"
	EventUnlock            = "Unlock"
	EventVote              = "Vote"
	EventProposalHeartbeat = "ProposalHeartbeat"
)

///////////////////////////////////////////////////////////////////////////////
// ENCODING / DECODING
///////////////////////////////////////////////////////////////////////////////

// implements events.EventData
type TMEventData interface {
	AssertIsTMEventData()
	// empty interface
}

func (_ EventDataNewBlock) AssertIsTMEventData()          {}
func (_ EventDataNewBlockHeader) AssertIsTMEventData()    {}
func (_ EventDataLog) AssertIsTMEventData()               {}
func (_ EventDataRoundState) AssertIsTMEventData()        {}
func (_ EventDataVote) AssertIsTMEventData()              {}
func (_ EventDataProposalHeartbeat) AssertIsTMEventData() {}
func (_ EventDataString) AssertIsTMEventData()            {}

func RegisterEventDatas() {
	ser.RegisterInterface((*TMEventData)(nil), nil)
	ser.RegisterConcrete(EventDataNewBlock{}, "event/NewBlock", nil)
	ser.RegisterConcrete(EventDataNewBlockHeader{}, "event/NewBlockHeader", nil)
	ser.RegisterConcrete(EventDataLog{}, "event/Log", nil)
	ser.RegisterConcrete(EventDataRoundState{}, "event/RoundState", nil)
	ser.RegisterConcrete(EventDataVote{}, "event/Vote", nil)
	ser.RegisterConcrete(EventDataProposalHeartbeat{}, "event/ProposalHeartbeat", nil)
	ser.RegisterConcrete(EventDataString(""), "event/ProposalString", nil)
}

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// light weight event for benchmarking
type EventDataNewBlockHeader struct {
	Header *Header `json:"header"`
}

type EventDataLog struct {
	Logs []*Log `json:"logs"`
}

type EventDataProposalHeartbeat struct {
	Heartbeat *Heartbeat
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height uint64 `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	// private, not exposed to websockets
	RoundState interface{} `json:"-" rlp:"-"`
}

type EventDataVote struct {
	Vote *Vote
}

type EventDataString string

///////////////////////////////////////////////////////////////////////////////
// PUBSUB
///////////////////////////////////////////////////////////////////////////////

const (
	// EventTypeKey is a reserved key, used to specify event type in tags.
	EventTypeKey = "tm.event"
	// TxHashKey is a reserved key, used to specify transaction's hash.
	// see EventBus#PublishEventTx
	TxHashKey = "tx.hash"
	// TxHeightKey is a reserved key, used to specify transaction block's height.
	// see EventBus#PublishEventTx
	TxHeightKey = "tx.height"
)

var (
	EventQueryBond              = QueryForEvent(EventBond)
	EventQueryUnbond            = QueryForEvent(EventUnbond)
	EventQueryRebond            = QueryForEvent(EventRebond)
	EventQueryDupeout           = QueryForEvent(EventDupeout)
	EventQueryFork              = QueryForEvent(EventFork)
	EventQueryNewBlock          = QueryForEvent(EventNewBlock)
	EventQueryNewBlockHeader    = QueryForEvent(EventNewBlockHeader)
	EventQueryNewRound          = QueryForEvent(EventNewRound)
	EventQueryNewRoundStep      = QueryForEvent(EventNewRoundStep)
	EventQueryTimeoutPropose    = QueryForEvent(EventTimeoutPropose)
	EventQueryCompleteProposal  = QueryForEvent(EventCompleteProposal)
	EventQueryPolka             = QueryForEvent(EventPolka)
	EventQueryUnlock            = QueryForEvent(EventUnlock)
	EventQueryLock              = QueryForEvent(EventLock)
	EventQueryRelock            = QueryForEvent(EventRelock)
	EventQueryTimeoutWait       = QueryForEvent(EventTimeoutWait)
	EventQueryVote              = QueryForEvent(EventVote)
	EventQueryProposalHeartbeat = QueryForEvent(EventProposalHeartbeat)
	EventQueryTx                = QueryForEvent(EventTx)
	EventQueryLog               = QueryForEvent(EventLog)
)

func EventQueryTxFor(tx Tx) tmpubsub.Query {
	return tmquery.MustParse(fmt.Sprintf("%s='%s' AND %s='%X'", EventTypeKey, EventTx, TxHashKey, tx.Hash()))
}

func QueryForEvent(eventType string) tmpubsub.Query {
	return tmquery.MustParse(fmt.Sprintf("%s='%s'", EventTypeKey, eventType))
}

// BlockEventPublisher publishes all block related events
type BlockEventPublisher interface {
	PublishEventNewBlock(block EventDataNewBlock) error
	PublishEventNewBlockHeader(header EventDataNewBlockHeader) error
	PublishEventLog(EventDataLog) error
}
