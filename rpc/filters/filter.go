// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package filters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/bloombits"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/types"
)

type Backend interface {
	EventBus() *types.EventBus
	HeaderByHeight(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)

	GetReceipts(ctx context.Context, blockNr uint64) types.Receipts

	BloomStatus() (uint64, uint64)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
}

// FilterCriteria represents a request to create a new filter.
type FilterCriteria struct {
	FromBlock *hexutil.Big     `json:"fromBlock"`
	ToBlock   *hexutil.Big     `json:"toBlock"`
	Addresses []common.Address `json:"addrs" gencodec:"required"`
	Topics    [][]common.Hash  `json:"topics" gencodec:"required"`
}

func (fc FilterCriteria) String() string {
	return fmt.Sprintf("{from=%v, to=%v, address=%v, topics=%v}", fc.FromBlock, fc.ToBlock, fc.Addresses, fc.Topics)
	//return fmt.Sprintf("{from=%s, to=%s}", fc.FromBlock.String(), fc.ToBlock.String())
}

// UnmarshalJSON sets *args fields with given data.
func (fc *FilterCriteria) UnmarshalJSON(data []byte) error {
	type input struct {
		// BlockHash *common.Hash     `json:"blockHash"`
		FromBlock *rpc.BlockNumber `json:"fromBlock"`
		ToBlock   *rpc.BlockNumber `json:"toBlock"`
		Addresses interface{}      `json:"address"`
		Topics    []interface{}    `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.FromBlock != nil {
		fc.FromBlock = (*hexutil.Big)(big.NewInt(raw.FromBlock.Int64()))
	}

	if raw.ToBlock != nil {
		fc.ToBlock = (*hexutil.Big)(big.NewInt(raw.ToBlock.Int64()))
	}

	fc.Addresses = []common.Address{}

	if raw.Addresses != nil {
		// raw.Address can contain a single address or an array of addresses
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr, err := decodeAddress(strAddr)
					if err != nil {
						return fmt.Errorf("invalid address at index %d: %v", i, err)
					}
					fc.Addresses = append(fc.Addresses, addr)
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr, err := decodeAddress(rawAddr)
			if err != nil {
				return fmt.Errorf("invalid address: %v", err)
			}
			fc.Addresses = []common.Address{addr}
		default:
			return errors.New("invalid addresses in query")
		}
	}

	// topics is an array consisting of strings and/or arrays of strings.
	// JSON null values are converted to common.Hash{} and ignored by the filter manager.
	if len(raw.Topics) > 0 {
		fc.Topics = make([][]common.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
				// ignore topic when matching logs

			case string:
				// match specific topic
				top, err := decodeTopic(topic)
				if err != nil {
					return err
				}
				fc.Topics[i] = []common.Hash{top}

			case []interface{}:
				// or case e.g. [null, "topic0", "topic1"]
				for _, rawTopic := range topic {
					if rawTopic == nil {
						// null component, match all
						fc.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed, err := decodeTopic(topic)
						if err != nil {
							return err
						}
						fc.Topics[i] = append(fc.Topics[i], parsed)
					} else {
						return fmt.Errorf("invalid topic(s)")
					}
				}
			default:
				return fmt.Errorf("invalid topic(s)")
			}
		}
	}

	return nil
}

func decodeAddress(s string) (common.Address, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.AddressLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for address", len(b), common.AddressLength)
	}
	return common.BytesToAddress(b), err
}

func decodeTopic(s string) (common.Hash, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.HashLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), common.HashLength)
	}
	return common.BytesToHash(b), err
}

type filterCriteriaMarshaling struct {
	FromBlock *hexutil.Big
	ToBlock   *hexutil.Big
}

// Filter can be used to retrieve and filter logs.
type Filter struct {
	backend Backend

	begin, end int64
	addresses  []common.Address
	topics     [][]common.Hash

	matcher *bloombits.Matcher
}

// New creates a new filter which uses a bloom filter on blocks to figure out whether
// a particular block is interesting or not.
func New(backend Backend, begin, end int64, addresses []common.Address, topics [][]common.Hash) *Filter {
	// Flatten the address and topic filter clauses into a single bloombits filter
	// system. Since the bloombits are not positional, nil topics are permitted,
	// which get flattened into a nil byte slice.
	var filters [][][]byte
	if len(addresses) > 0 {
		filter := make([][]byte, len(addresses))
		for i, address := range addresses {
			filter[i] = address.Bytes()
		}
		filters = append(filters, filter)
	}
	for _, topicList := range topics {
		filter := make([][]byte, len(topicList))
		for i, topic := range topicList {
			filter[i] = topic.Bytes()
		}
		filters = append(filters, filter)
	}
	// Assemble and return the filter
	size, _ := backend.BloomStatus()

	return &Filter{
		backend:   backend,
		begin:     begin,
		end:       end,
		addresses: addresses,
		topics:    topics,
		matcher:   bloombits.NewMatcher(size, filters),
	}
}

// Logs searches the blockchain for matching log entries, returning all from the
// first block that contains matches, updating the start of the filter accordingly.
func (f *Filter) Logs(ctx context.Context) ([]*types.Log, error) {
	// Figure out the limits of the filter range
	header, _ := f.backend.HeaderByHeight(ctx, rpc.LatestBlockNumber)
	if header == nil {
		return nil, nil
	}
	head := header.Height

	if f.begin == -1 {
		f.begin = int64(head)
	}
	end := uint64(f.end)
	if f.end == -1 {
		end = head
	}
	// Gather all indexed logs, and finish with non indexed ones
	var (
		logs []*types.Log
		err  error
	)
	size, sections := f.backend.BloomStatus()
	if indexed := sections * size; indexed > uint64(f.begin) {
		if indexed > end {
			logs, err = f.indexedLogs(ctx, end)
		} else {
			logs, err = f.indexedLogs(ctx, indexed-1)
		}
		if err != nil {
			return logs, err
		}
	}
	rest, err := f.unindexedLogs(ctx, end)
	logs = append(logs, rest...)
	return logs, err
}

// indexedLogs returns the logs matching the filter criteria based on the bloom
// bits indexed available locally or via the network.
func (f *Filter) indexedLogs(ctx context.Context, end uint64) ([]*types.Log, error) {
	// Create a matcher session and request servicing from the backend
	matches := make(chan uint64, 64)

	session, err := f.matcher.Start(ctx, uint64(f.begin), end, matches)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	f.backend.ServiceFilter(ctx, session)

	// Iterate over the matches until exhausted or context closed
	var logs []*types.Log

	for {
		select {
		case number, ok := <-matches:
			// Abort if all matches have been fulfilled
			if !ok {
				err := session.Error()
				if err == nil {
					f.begin = int64(end) + 1
				}
				return logs, err
			}
			f.begin = int64(number) + 1

			// Retrieve the suggested block and pull any truly matching logs
			header, err := f.backend.HeaderByHeight(ctx, rpc.BlockNumber(number))
			if header == nil || err != nil {
				return logs, err
			}
			found, err := f.checkMatches(ctx, header)
			if err != nil {
				return logs, err
			}
			logs = append(logs, found...)

		case <-ctx.Done():
			return logs, ctx.Err()
		}
	}
}

// indexedLogs returns the logs matching the filter criteria based on raw block
// iteration and bloom matching.
func (f *Filter) unindexedLogs(ctx context.Context, end uint64) ([]*types.Log, error) {
	var logs []*types.Log

	for ; f.begin <= int64(end); f.begin++ {
		header, err := f.backend.HeaderByHeight(ctx, rpc.BlockNumber(f.begin))
		if header == nil || err != nil {
			return logs, err
		}
		if bloomFilter(header.Bloom(), f.addresses, f.topics) {
			found, err := f.checkMatches(ctx, header)
			if err != nil {
				return logs, err
			}
			logs = append(logs, found...)
		}
	}
	return logs, nil
}

// checkMatches checks if the receipts belonging to the given header contain any log events that
// match the filter criteria. This function is called when the bloom filter signals a potential match.
func (f *Filter) checkMatches(ctx context.Context, header *types.Header) (logs []*types.Log, err error) {
	// Get the logs of the block
	receipts := f.backend.GetReceipts(ctx, header.Height)
	var unfiltered []*types.Log
	for _, receipt := range receipts {
		unfiltered = append(unfiltered, receipt.Logs...)
	}
	logs = filterLogs(unfiltered, nil, nil, f.addresses, f.topics)
	if len(logs) > 0 {
		return logs, nil
	}
	return nil, nil
}

func includes(addresses []common.Address, a common.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}

// filterLogs creates a slice of logs matching the given criteria.
func filterLogs(logs []*types.Log, fromBlock, toBlock *big.Int, addresses []common.Address, topics [][]common.Hash) []*types.Log {
	var ret []*types.Log
Logs:
	for _, log := range logs {
		if fromBlock != nil && fromBlock.Int64() >= 0 && fromBlock.Uint64() > log.BlockNumber {
			continue
		}
		if toBlock != nil && toBlock.Int64() >= 0 && toBlock.Uint64() < log.BlockNumber {
			continue
		}

		if len(addresses) > 0 && !includes(addresses, log.Address) {
			continue
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(log.Topics) {
			continue Logs
		}
		for i, topics := range topics {
			match := len(topics) == 0 // empty rule set == wildcard
			for _, topic := range topics {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				continue Logs
			}
		}
		ret = append(ret, log)
	}
	return ret
}

func bloomFilter(bloom types.Bloom, addresses []common.Address, topics [][]common.Hash) bool {
	if len(addresses) > 0 {
		var included bool
		for _, addr := range addresses {
			if types.BloomLookup(bloom, addr) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, sub := range topics {
		included := len(sub) == 0 // empty rule set == wildcard
		for _, topic := range sub {
			if types.BloomLookup(bloom, topic) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	return true
}
