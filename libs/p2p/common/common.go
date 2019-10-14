// Copyright 2019 The go-ethereum Authors
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

package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/p2p/netutil"
)

const (
	// HashLength is the expected length of the hash
	HashLength = 32
	// AddressLength is the expected length of the address
	AddressLength = 20
)

type Hash [HashLength]byte

// ID is a unique identifier for each node.
type NodeID [32]byte //crypto.Keccak256Hash(pubkey)

func (n NodeID) Bytes() []byte {
	return n[:]
}

func (n *NodeID) Copy(buffer []byte) {
	copy(n[:], buffer)
}

// ID prints as a long hexadecimal number.
func (n NodeID) String() string {
	return hexutil.Encode(n[:])
}

func (n NodeID) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"0x%x"`, n[:])), nil
}

func (n *NodeID) UnmarshalJSON(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("%s is not a hex string", data)
	}
	data = data[3 : len(data)-1]
	dec := make([]byte, len(data)/2)
	if _, err := hex.Decode(dec, data); err != nil {
		return err
	}
	n.Copy(dec[:])
	return nil
}

type Node struct {
	IP       net.IP `json:"ip"`       // len 4 for IPv4 or 16 for IPv6
	UDP_Port uint16 `json:"udp_port"` // port numbers
	TCP_Port uint16 `json:"tcp_port"` // port numbers
	ID       NodeID `json:"id"`       // the node's public key
}

// Incomplete returns true for nodes with no IP address.
func (n *Node) Incomplete() bool {
	return n.IP == nil || n.ID.Bytes() == nil
}

// ValidateComplete checks whether n has a valid IP and UDP port.
func (n *Node) ValidateComplete() error {
	if n.Incomplete() {
		return errors.New("missing IP address or ID")
	}
	ip := n.IP
	if ip.IsMulticast() || ip.IsUnspecified() {
		return errors.New("invalid IP (multicast/unspecified)")
	}
	// Validate the node key (on curve, etc.).
	return nil
}

type UDPConn interface {
	ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error)
	WriteToUDP(b []byte, addr *net.UDPAddr) (n int, err error)
	Close() error
	LocalAddr() net.Addr
}

type DiscoverTable interface {
	Start()
	Stop()
	GetMaxDialOutNum() int                        //Maximum number of connections to be actively connected outward
	GetMaxConNumFromCache() int                   //Get the maximum number of nodes from cache
	LookupRandom() []*Node                        //Get some nodes in real time from the network
	ReadRandomNodes([]*Node, map[string]bool) int //Get some random nodes from local memory
	IsDhtTable() bool
}

type P2pDBManager interface {
	QuerySeeds(n int, maxAge time.Duration) []*Node
	LastPingReceived(id NodeID, ip net.IP) time.Time
	LastPongReceived(id NodeID, ip net.IP) time.Time
	UpdateNode(node *Node) //Store Node in DB
	UpdateLastPingReceived(id NodeID, ip net.IP, instance time.Time)
	UpdateLastPongReceived(id NodeID, ip net.IP, instance time.Time)
	UpdateFindFails(id NodeID, ip net.IP, fails int)
	FindFails(id NodeID, ip net.IP) int
	Close()
}

// Config holds Table-related settings.
type Config struct {
	// These settings are required and configure the UDP listener:
	PrivateKey crypto.PrivKey

	// These settings are optional:
	NetRestrict *netutil.Netlist // network whitelist
	SeedNodes   []*Node          // list of bootstrap nodes
}

// ReadPacket is a packet that couldn't be handled. Those packets are sent to the unhandled
// channel if configured.
type ReadPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

func TransPubKeyToStringID(pubKey crypto.PubKey) string {
	return hex.EncodeToString(crypto.Keccak256Hash(pubKey.Bytes()).Bytes())
}

func TransPubKeyToNodeID(pubKey crypto.PubKey) NodeID {
	var id = &NodeID{}
	id.Copy(crypto.Keccak256Hash(pubKey.Bytes()).Bytes())
	return *id
}

func TransPkbyteToNodeID(pubKey []byte) NodeID {
	var id = &NodeID{}
	id.Copy(crypto.Keccak256Hash(pubKey).Bytes())
	return *id
}

func TransNodeIDToString(nodeID NodeID) string {
	return hex.EncodeToString(nodeID.Bytes())
}
