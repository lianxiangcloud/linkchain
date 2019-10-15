// Copyright 2015 The go-ethereum Authors
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

package discover

import (
	"math/bits"
	"net"
	"time"

	common "github.com/lianxiangcloud/linkchain/libs/p2p/common"

	"fmt"
)

const (
	defaultSeeds = 3
)

// node represents a host on the network.
// The fields of Node may not be modified.
type node struct {
	havePublicAddr bool //havePublicAddr means this node upnp success or is in public network
	common.Node
	addedAt        time.Time // time when the node was added to the table
	livenessChecks uint      // how often liveness was checked
}

// LogDist returns the logarithmic distance between a and b, log2(a ^ b).
func LogDist(a, b common.NodeID) int {
	lz := 0
	for i := range a {
		x := a[i] ^ b[i]
		if x == 0 {
			lz += 8
		} else {
			lz += bits.LeadingZeros8(x)
			break
		}
	}
	return len(a)*8 - lz
}

// DistCmp compares the distances a->target and b->target.
// Returns -1 if a is closer to target, 1 if b is closer to target
// and 0 if they are equal.
func DistCmp(target, a, b common.NodeID) int {
	for i := range target {
		da := a[i] ^ target[i]
		db := b[i] ^ target[i]
		if da > db {
			return 1
		} else if da < db {
			return -1
		}
	}
	return 0
}

func wrapNode(n *common.Node) *node {
	return &node{havePublicAddr: false, Node: *n}
}

func wrapNodes(ns []*common.Node) []*node {
	result := make([]*node, len(ns))
	for i, n := range ns {
		result[i] = wrapNode(n)
	}
	return result
}

func unwrapNode(n *node) *common.Node {
	return &n.Node
}

func unwrapNodes(ns []*node) []*common.Node {
	result := make([]*common.Node, len(ns))
	for i, n := range ns {
		result[i] = unwrapNode(n)
	}
	return result
}

func (n *node) addr() *net.UDPAddr {
	return &net.UDPAddr{IP: n.IP, Port: int(n.UDP_Port)}
}

func (n *node) String() string {
	return fmt.Sprintf("IP:%v udp_port:%v tcp_port:%v ID:%v", n.Node.IP.String(), n.Node.UDP_Port, n.Node.TCP_Port, n.Node.ID.String())
}
