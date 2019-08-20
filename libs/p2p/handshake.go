package p2p

import (
	"net"
	"time"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

var (
	//HandShakeFunc is the gloable fuction of p2p handshake
	HandShakeFunc = defaultHandshakeTimeout
)

func defaultHandshakeTimeout(
	conn net.Conn,
	ourNodeInfo NodeInfo,
	timeout time.Duration,
	isInCon bool,
) (peerNodeInfo NodeInfo, err error) {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return peerNodeInfo, cmn.ErrorWrap(err, "Error setting deadline")
	}

	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			_, err = ser.EncodeWriterWithType(conn, ourNodeInfo)
			return
		},
		func(_ int) (val interface{}, err error, abort bool) {
			_, err = ser.DecodeReaderWithType(
				conn,
				&peerNodeInfo,
				int64(MaxNodeInfoSize()),
			)
			return
		},
	)
	if err := trs.FirstError(); err != nil {
		return peerNodeInfo, cmn.ErrorWrap(err, "Error during handshake")
	}

	// Remove deadline
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return peerNodeInfo, cmn.ErrorWrap(err, "Error removing deadline")
	}

	return peerNodeInfo, nil
}
