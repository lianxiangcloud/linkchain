package conn

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fmt"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

var (
	logger = log.Root()
)

func init() {
	logger.SetHandler(log.StdoutHandler)
}

var (
	num = 10
)

const maxPingPongPacketSize = 1024 // bytes

func createTestMConnection(conn net.Conn) *MConnection {
	onReceive := func(chID byte, msgBytes []byte) {
	}
	onError := func(r interface{}) {
	}
	c := createMConnectionWithCallbacks(conn, onReceive, onError)
	c.SetLogger(log.Test())
	return c
}

func createMConnectionWithCallbacks(conn net.Conn, onReceive func(chID byte, msgBytes []byte), onError func(r interface{})) *MConnection {
	cfg := DefaultMConnConfig()
	cfg.PingInterval = 90 * time.Millisecond
	cfg.PongTimeout = 45 * time.Millisecond
	chDescs := []*ChannelDescriptor{
		&ChannelDescriptor{ID: 0x01, Priority: 1, SendQueueCapacity: num},
		&ChannelDescriptor{ID: 0x02, Priority: 1, SendQueueCapacity: num},
		{ID: 0x30, Priority: 1, SendQueueCapacity: num},
	}
	c := NewMConnectionWithConfig(conn, chDescs, onReceive, onError, cfg)
	c.SetLogger(log.Test())
	return c
}

func TestMConnectionSend(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	mconn := createTestMConnection(client)
	//mconn.conn.SetDeadline(time.Now().Add(time.Duration(sleepTime) * time.Second))
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()
	msg := make([]byte, 1024)
	//msg := make([]byte, 1024)
	for i := 0; i < num; i++ {
		result := mconn.TrySend(0x01, msg)
		assert.True(t, result)
		//mconn.TrySend(0x01, msg)
	}

	//result := mconn.TrySend(0x01, msg)
	//logger.Debug("TrySend", "result", result)
	//assert.True(t, result)

	//var packet Packet
	//time.Sleep(time.Second * time.Duration(sleepTime))
	var packet Packet
	var _n int64
	mconn2 := createTestMConnection(server)
	//defer mconn2.Stop()
	logger.Debug("DecodeReaderWithType", "int64(mconn.maxPacketMsgSize())", int64(mconn.maxPacketMsgSize()))
	for i := 0; i < num; i++ {
		_, err = ser.EncodeWriterWithType(server, PacketPong{})
		require.Nil(t, err)

		mconn2.recvMonitor.Limit(mconn.maxPacketMsgSize(), 10, true)
		_n, err = ser.DecodeReaderWithType(mconn2.bufConnReader, &packet, int64(mconn.maxPacketMsgSize()))
		//_n, err = ser.DecodeReaderWithType(mconn2.bufConnReader, &packet, 31*1024)
		if err != nil {
			t.Error(err)
		}
		switch pkt := packet.(type) {
		case PacketMsg:
			logger.Debug("DecodeReaderWithType success", "_n", _n, "len(msg)", len(msg))
		default:
			t.Error(fmt.Errorf("pkt:%v unexpect", pkt))
		}
	}

	logger.Debug("DecodeReaderWithType", "_n", _n)
}

func TestMConnectionReceive(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn1 := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn1.Start()
	require.Nil(t, err)
	defer mconn1.Stop()

	mconn2 := createTestMConnection(server)
	err = mconn2.Start()
	require.Nil(t, err)
	defer mconn2.Stop()

	msg := []byte("Cyclops")
	assert.True(t, mconn2.Send(0x01, msg))

	select {
	case receivedBytes := <-receivedCh:
		assert.Equal(t, []byte(msg), receivedBytes)
	case err := <-errorsCh:
		t.Fatalf("Expected %s, got %+v", msg, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Did not receive %s message in 500ms", msg)
	}
}

func TestMConnectionStatus(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	status := mconn.Status()
	assert.NotNil(t, status)
	assert.Zero(t, status.Channels[0].SendQueueSize)
}

func TestMConnectionPongTimeoutResultsInError(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	serverGotPing := make(chan struct{})
	go func() {
		// read ping
		var pkt PacketPing
		_, err = ser.DecodeReaderWithType(server, &pkt, maxPingPongPacketSize)
		//_, err = ser.DecodeReader(server, &pkt, maxPingPongPacketSize)
		logger.Debug("DecodeReader", "err", err)
		assert.Nil(t, err)
		serverGotPing <- struct{}{}
	}()
	<-serverGotPing

	pongTimerExpired := mconn.config.PongTimeout + 20*time.Millisecond
	select {
	case msgBytes := <-receivedCh:
		t.Fatalf("Expected error, but got %v", msgBytes)
	case err := <-errorsCh:
		assert.NotNil(t, err)
	case <-time.After(pongTimerExpired):
		t.Fatalf("Expected to receive error after %v", pongTimerExpired)
	}
}

func TestMConnectionMultiplePongsInTheBeginning(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	// sending 3 pongs in a row (abuse)
	//_, err = server.Write(ser.MustEncodeToBytes(PacketPong{}))
	_, err = ser.EncodeWriterWithType(server, PacketPong{})
	require.Nil(t, err)
	//_, err = server.Write(ser.MustEncodeToBytes(PacketPong{}))
	_, err = ser.EncodeWriterWithType(server, PacketPong{})
	require.Nil(t, err)
	//_, err = server.Write(ser.MustEncodeToBytes(PacketPong{}))
	_, err = ser.EncodeWriterWithType(server, PacketPong{})
	require.Nil(t, err)

	serverGotPing := make(chan struct{})
	go func() {
		// read ping (one byte)
		var packet, err = Packet(nil), error(nil)
		_, err = ser.DecodeReader(server, &packet, maxPingPongPacketSize)
		require.Nil(t, err)
		serverGotPing <- struct{}{}
		// respond with pong
		//_, err = server.Write(ser.MustEncodeToBytes(PacketPong{}))
		_, err = ser.EncodeWriterWithType(server, PacketPong{})
		require.Nil(t, err)
	}()
	<-serverGotPing

	pongTimerExpired := mconn.config.PongTimeout + 20*time.Millisecond
	select {
	case msgBytes := <-receivedCh:
		t.Fatalf("Expected no data, but got %v", msgBytes)
	case err := <-errorsCh:
		t.Fatalf("Expected no error, but got %v", err)
	case <-time.After(pongTimerExpired):
		assert.True(t, mconn.IsRunning())
	}
}

func TestMConnectionMultiplePings(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	// sending 3 pings in a row (abuse)
	_, err = ser.EncodeWriterWithType(server, PacketPing{})
	//_, err = ser.EncodeWriterWithType(server, PacketPong{})
	require.Nil(t, err)
	var pkt PacketPong
	//_, err = ser.DecodeReader(server, &pkt, maxPingPongPacketSize)
	_, err = ser.DecodeReaderWithType(server, &pkt, maxPingPongPacketSize)
	require.Nil(t, err)
	_, err = ser.EncodeWriterWithType(server, PacketPing{})
	//_, err = ser.EncodeWriterWithType(server, PacketPong{})
	require.Nil(t, err)
	_, err = ser.DecodeReaderWithType(server, &pkt, maxPingPongPacketSize)
	//_, err = ser.DecodeReader(server, &pkt, maxPingPongPacketSize)
	require.Nil(t, err)
	_, err = ser.EncodeWriterWithType(server, PacketPing{})
	//_, err = ser.EncodeWriterWithType(server, PacketPong{})
	require.Nil(t, err)
	_, err = ser.DecodeReaderWithType(server, &pkt, maxPingPongPacketSize)
	//_, err = ser.DecodeReader(server, &pkt, maxPingPongPacketSize)
	require.Nil(t, err)

	assert.True(t, mconn.IsRunning())
}

func TestMConnectionPingPongs(t *testing.T) {
	// check that we are not leaking any go-routines
	defer leaktest.CheckTimeout(t, 10*time.Second)()

	server, client := net.Pipe()

	defer server.Close()
	defer client.Close()

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	serverGotPing := make(chan struct{})
	go func() {
		// read ping
		var pkt PacketPing
		_, err = ser.DecodeReader(server, &pkt, maxPingPongPacketSize)
		require.Nil(t, err)
		serverGotPing <- struct{}{}
		// respond with pong
		_, err = server.Write(ser.MustEncodeToBytes(PacketPong{}))
		require.Nil(t, err)

		time.Sleep(mconn.config.PingInterval)

		// read ping
		_, err = ser.DecodeReader(server, &pkt, maxPingPongPacketSize)
		require.Nil(t, err)
		// respond with pong
		_, err = server.Write(ser.MustEncodeToBytes(PacketPong{}))
		require.Nil(t, err)
	}()
	<-serverGotPing

	pongTimerExpired := (mconn.config.PongTimeout + 20*time.Millisecond) * 2
	select {
	case msgBytes := <-receivedCh:
		t.Fatalf("Expected no data, but got %v", msgBytes)
	case err := <-errorsCh:
		t.Fatalf("Expected no error, but got %v", err)
	case <-time.After(2 * pongTimerExpired):
		assert.True(t, mconn.IsRunning())
	}
}

func TestMConnectionStopsAndReturnsError(t *testing.T) {
	server, client := NetPipe()
	defer server.Close() // nolint: errcheck
	defer client.Close() // nolint: errcheck

	receivedCh := make(chan []byte)
	errorsCh := make(chan interface{})
	onReceive := func(chID byte, msgBytes []byte) {
		receivedCh <- msgBytes
	}
	onError := func(r interface{}) {
		errorsCh <- r
	}
	mconn := createMConnectionWithCallbacks(client, onReceive, onError)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	if err := client.Close(); err != nil {
		t.Error(err)
	}

	select {
	case receivedBytes := <-receivedCh:
		t.Fatalf("Expected error, got %v", receivedBytes)
	case err := <-errorsCh:
		assert.NotNil(t, err)
		assert.False(t, mconn.IsRunning())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Did not receive error in 500ms")
	}
}

func newClientAndServerConnsForReadErrors(t *testing.T, chOnErr chan struct{}) (*MConnection, *MConnection) {
	server, client := NetPipe()

	onReceive := func(chID byte, msgBytes []byte) {}
	onError := func(r interface{}) {}

	// create client conn with two channels
	chDescs := []*ChannelDescriptor{
		{ID: 0x01, Priority: 1, SendQueueCapacity: 1},
		{ID: 0x02, Priority: 1, SendQueueCapacity: 1},
	}
	mconnClient := NewMConnection(client, chDescs, onReceive, onError)
	mconnClient.SetLogger(log.Test().With("module", "client"))
	err := mconnClient.Start()
	require.Nil(t, err)

	// create server conn with 1 channel
	// it fires on chOnErr when there's an error
	serverLogger := log.Test().With("module", "server")
	onError = func(r interface{}) {
		chOnErr <- struct{}{}
	}
	mconnServer := createMConnectionWithCallbacks(server, onReceive, onError)
	mconnServer.SetLogger(serverLogger)
	err = mconnServer.Start()
	require.Nil(t, err)
	return mconnClient, mconnServer
}

func expectSend(ch chan struct{}) bool {
	after := time.After(time.Second * 5)
	select {
	case <-ch:
		return true
	case <-after:
		return false
	}
}

func TestMConnectionReadErrorBadEncoding(t *testing.T) {
	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	client := mconnClient.conn

	// send badly encoded msgPacket
	bz := ser.MustEncodeToBytes(PacketMsg{})
	bz[4] += 0x01 // Invalid prefix bytes.

	// Write it.
	_, err := client.Write(bz)
	assert.Nil(t, err)
	assert.True(t, expectSend(chOnErr), "badly encoded msgPacket")
}

func TestMConnectionReadErrorUnknownChannel(t *testing.T) {
	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	msg := []byte("Ant-Man")

	// fail to send msg on channel unknown by client
	assert.False(t, mconnClient.Send(0x03, msg))

	// send msg on channel unknown by the server.
	// should cause an error
	assert.True(t, mconnClient.Send(0x02, msg))
	assert.True(t, expectSend(chOnErr), "unknown channel")
}

func TestMConnectionReadErrorLongMessage(t *testing.T) {
	chOnErr := make(chan struct{})
	chOnRcv := make(chan struct{})

	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	mconnServer.onReceive = func(chID byte, msgBytes []byte) {
		chOnRcv <- struct{}{}
	}

	client := mconnClient.conn

	// send msg thats just right
	var err error
	var buf = new(bytes.Buffer)
	var packet = PacketMsg{
		ChannelID: 0x01,
		EOF:       1,
		Bytes:     make([]byte, mconnClient.config.MaxPacketMsgPayloadSize),
	}
	_, err = ser.EncodeWriter(buf, packet)
	assert.Nil(t, err)
	_, err = client.Write(buf.Bytes())
	assert.Nil(t, err)
	assert.True(t, expectSend(chOnRcv), "msg just right")
	assert.False(t, expectSend(chOnErr), "msg just right")

	// send msg thats too long
	buf = new(bytes.Buffer)
	packet = PacketMsg{
		ChannelID: 0x01,
		EOF:       1,
		Bytes:     make([]byte, mconnClient.config.MaxPacketMsgPayloadSize+100),
	}
	_, err = ser.EncodeWriter(buf, packet)
	assert.Nil(t, err)
	_, err = client.Write(buf.Bytes())
	assert.NotNil(t, err)
	assert.False(t, expectSend(chOnRcv), "msg too long")
	assert.True(t, expectSend(chOnErr), "msg too long")
}

func TestMConnectionReadErrorUnknownMsgType(t *testing.T) {
	chOnErr := make(chan struct{})
	mconnClient, mconnServer := newClientAndServerConnsForReadErrors(t, chOnErr)
	defer mconnClient.Stop()
	defer mconnServer.Stop()

	// send msg with unknown msg type
	err := error(nil)
	_, err = mconnClient.conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	assert.Nil(t, err)
	assert.True(t, expectSend(chOnErr), "unknown msg type")
}

func TestMConnectionTrySend(t *testing.T) {
	server, client := NetPipe()
	defer server.Close()
	defer client.Close()

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()

	msg := []byte("Semicolon-Woman")
	resultCh := make(chan string, 2)
	assert.True(t, mconn.TrySend(0x01, msg))
	server.Read(make([]byte, len(msg)))
	assert.True(t, mconn.CanSend(0x01))
	assert.True(t, mconn.TrySend(0x01, msg))
	assert.False(t, mconn.CanSend(0x01))
	go func() {
		mconn.TrySend(0x01, msg)
		resultCh <- "TrySend"
	}()
	assert.False(t, mconn.CanSend(0x01))
	assert.False(t, mconn.TrySend(0x01, msg))
	assert.Equal(t, "TrySend", <-resultCh)
}

func TestErrorWrap(t *testing.T) {
	err := cmn.ErrorWrap("panic test", "recovered panic in MConnection")
	t.Fatalf("err:%v", err)
}

func TestSend(t *testing.T) {
	server, client := NetPipe()
	defer server.Close()
	defer client.Close()

	mconn := createTestMConnection(client)
	err := mconn.Start()
	require.Nil(t, err)
	defer mconn.Stop()
	var data []byte
	_ = mconn.Send(48, data)
	time.Sleep(time.Second * 2)
}
