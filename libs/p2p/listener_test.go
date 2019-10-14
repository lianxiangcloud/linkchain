package p2p

import (
	"fmt"
	"testing"

	"bytes"

	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/stretchr/testify/require"
)

func TestListener(t *testing.T) {
	logger := log.Root()
	logger.SetHandler(log.StdoutHandler)
	// Create a listener
	listenPort := 50000
	listenAddr := fmt.Sprintf(":%d", listenPort)
	externalAddrString1 := ""
	//externalAddrString2 := "121.58.96.78"
	listener, udpCon, _ := NewDefaultListener(types.NodePeer, listenAddr, externalAddrString1, logger)
	require.Equal(t, listenPort, int(listener.ExternalAddress().Port))
	_, udpPort := SplitHostPort(udpCon.LocalAddr().String())
	require.Equal(t, listenPort, udpPort)
	//bind the same port
	listener, udpCon, _ = NewDefaultListener(types.NodePeer, listenAddr, externalAddrString1, logger)
	require.Equal(t, listenPort+1, int(listener.ExternalAddress().Port))
	_, udpPort = SplitHostPort(udpCon.LocalAddr().String())
	require.Equal(t, listenPort+1, udpPort)
	//fullListenAddrString is empty
	listener, udpCon, _ = NewDefaultListener(types.NodePeer, "", externalAddrString1, logger)
	require.Equal(t, DefaultExternalPort, int(listener.ExternalAddress().Port))
	_, udpPort = SplitHostPort(udpCon.LocalAddr().String())
	require.Equal(t, DefaultExternalPort, udpPort)
	//OutValidator
	listener, udpCon, _ = NewDefaultListener(types.NodeValidator, "", externalAddrString1, logger)
	require.Equal(t, DefaultExternalPort+1, int(listener.ExternalAddress().Port))
	flag := false
	if udpCon == nil {
		flag = true
	}
	require.Equal(t, flag, true)
	//OutPeer have externalAddr
	var host string
	listenPort = 51000
	listenAddr = fmt.Sprintf(":%d", listenPort)
	externalAddrString2 := "127.0.0.1"
	listener, udpCon, _ = NewDefaultListener(types.NodePeer, listenAddr, externalAddrString2, logger)
	require.Equal(t, listenPort, int(listener.ExternalAddress().Port))
	require.Equal(t, externalAddrString2, listener.ExternalAddress().IP.String())
	require.Equal(t, externalAddrString2, listener.ExternalAddressHost())
	host, udpPort = SplitHostPort(udpCon.LocalAddr().String())
	require.Equal(t, listenPort, udpPort)
	require.Equal(t, externalAddrString2, host)
	//worng externalAddr
	listenPort = listenPort + 1
	listenAddr = fmt.Sprintf(":%d", listenPort)
	externalAddrString2 = "137.0.0.1"
	listener, udpCon, _ = NewDefaultListener(types.NodePeer, listenAddr, externalAddrString2, logger)
	flag = false
	if udpCon == nil {
		flag = true
	}
	require.Equal(t, flag, true)
	require.Equal(t, externalAddrString2, listener.ExternalAddress().IP.String())
	require.Equal(t, listenPort, int(listener.ExternalAddress().Port))
	//NodeTypeOutValidator have externalAddr
	listener, udpCon, _ = NewDefaultListener(types.NodeValidator, listenAddr, externalAddrString2, logger)
	flag = false
	if udpCon == nil {
		flag = true
	}
	require.Equal(t, flag, true)
	require.Equal(t, externalAddrString2, listener.ExternalAddress().IP.String())
	require.Equal(t, listenPort+1, int(listener.ExternalAddress().Port))

	externalAddrString2 = "127.0.0.1"
	listener, udpCon, _ = NewDefaultListener(types.NodeValidator, "", externalAddrString2, logger)
	flag = false
	if udpCon == nil {
		flag = true
	}
	require.Equal(t, flag, true)
	require.Equal(t, externalAddrString2, listener.ExternalAddress().IP.String())
	require.Equal(t, DefaultExternalPort+2, int(listener.ExternalAddress().Port))

	// Dial the listener
	lAddr := listener.ExternalAddress()
	connOut, err := lAddr.Dial()
	if err != nil {
		t.Fatalf("Could not connect to listener address %v", lAddr)
	} else {
		t.Logf("Created a connection to listener address %v", lAddr)
	}
	connIn, ok := <-listener.Connections()
	if !ok {
		t.Fatalf("Could not get inbound connection from listener")
	}

	msg := []byte("hi!")
	go func() {
		_, err := connIn.Write(msg)
		if err != nil {
			t.Error(err)
		}
	}()
	b := make([]byte, 32)
	n, err := connOut.Read(b)
	if err != nil {
		t.Fatalf("Error reading off connection: %v", err)
	}

	b = b[:n]
	if !bytes.Equal(msg, b) {
		t.Fatalf("Got %s, expected %s", b, msg)
	}

	// Close the server, no longer needed.
	listener.Stop()
}
