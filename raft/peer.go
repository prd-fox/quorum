package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	"net"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
)

// Serializable information about a Peer. Sufficient to build `etcdRaft.Peer`
// or `enode.Node`.
// As NodeId is mainly used to derive the `ecdsa.pubkey` to build `enode.Node` it is kept as [64]byte instead of ID [32]byte used by `enode.Node`.
type Address struct {
	RaftId   uint16        `json:"raftId"`
	NodeId   enode.EnodeID `json:"nodeId"`
	Ip       net.IP        `json:"-"`
	P2pPort  enr.TCP       `json:"p2pPort"`
	RaftPort enr.RaftPort  `json:"raftPort"`

	Hostname string `json:"hostname"`

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `json:"-" rlp:"tail"`
}

func (addr Address) MarshalJSON() ([]byte, error) {
	type address struct {
		RaftId   uint16        `json:"raftId"`
		NodeId   enode.EnodeID `json:"nodeId"`
		Ip       net.IP        `json:"ip,omitempty"`
		P2pPort  enr.TCP       `json:"p2pPort"`
		RaftPort enr.RaftPort  `json:"raftPort"`
		Hostname string        `json:"hostname"`
	}
	var enc address
	enc.RaftId = addr.RaftId
	enc.NodeId = addr.NodeId
	enc.P2pPort = addr.P2pPort
	enc.RaftPort = addr.RaftPort
	enc.Hostname = addr.Hostname
	if ip := net.ParseIP(addr.Hostname); ip != nil && ip.To4() != nil {
		enc.Ip = ip.To4()
	}
	return json.Marshal(&enc)
}

func newAddress(raftId uint16, raftPort int, node *enode.Node, withHostname bool) *Address {
	// derive 64 byte nodeID from 128 byte enodeID
	id, err := enode.RaftHexID(node.EnodeID())
	if err != nil {
		panic(err)
	}
	if withHostname {
		return &Address{
			RaftId:   raftId,
			NodeId:   id,
			Ip:       nil,
			P2pPort:  enr.TCP(node.TCP()),
			RaftPort: enr.RaftPort(raftPort),
			Hostname: node.Host(),
		}
	}
	return &Address{
		RaftId:   raftId,
		NodeId:   id,
		Ip:       nil,
		P2pPort:  enr.TCP(node.TCP()),
		RaftPort: enr.RaftPort(raftPort),
		Hostname: node.IP().String(),
	}
}

// A peer that we're connected to via both raft's http transport, and ethereum p2p
type Peer struct {
	address *Address    // For raft transport
	p2pNode *enode.Node // For ethereum transport
}

// RLP Address encoding, for transport over raft and storage in LevelDB.
func (addr *Address) toBytes(withHostname bool) []byte {
	var toEncode interface{}

	if withHostname {
		toEncode = addr
	} else {
		toEncode = []interface{}{addr.RaftId, addr.NodeId, net.ParseIP(addr.Hostname), addr.P2pPort, addr.RaftPort}
	}

	buffer, err := rlp.EncodeToBytes(toEncode)
	if err != nil {
		panic(fmt.Sprintf("error: failed to RLP-encode Address: %s", err.Error()))
	}
	return buffer
}

func bytesToAddress(input []byte) *Address {
	//try the new format first
	addr := new(Address)
	streamNew := rlp.NewStream(bytes.NewReader(input), 0)
	if err := streamNew.Decode(addr); err == nil {
		return addr
	}

	//else try the old format
	var temp struct {
		RaftId   uint16
		NodeId   enode.EnodeID
		Ip       net.IP
		P2pPort  enr.TCP
		RaftPort enr.RaftPort
	}

	streamOld := rlp.NewStream(bytes.NewReader(input), 0)
	if err := streamOld.Decode(&temp); err != nil {
		log.Fatalf("failed to RLP-decode Address: %v", err)
	}

	return &Address{
		RaftId:   temp.RaftId,
		NodeId:   temp.NodeId,
		Ip:       nil,
		P2pPort:  temp.P2pPort,
		RaftPort: temp.RaftPort,
		Hostname: temp.Ip.String(),
	}
}
