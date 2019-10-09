package daemon

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/bootnode"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/wallet/config"
)

// client return a http client to peer rpc
type client struct {
	Addrs         []string
	AddrIdx       int
	HTTPClient    *http.Client
	NC            string
	Origin        string
	Appversion    string
	DaemonVersion string
	lock          sync.Mutex
}

var gDaemonClient *client

const (
	defaultDialTimeout = 10 * time.Second
	keepAliveInterval  = 30 * time.Second
)

// InitClient init Client with config.DaemonConfig
func InitClient(daemonConfig *config.DaemonConfig, walletVersion string, logger log.Logger) {
	gDaemonClient = &client{
		Addrs:         daemonConfig.PeerRPC,
		AddrIdx:       0,
		NC:            daemonConfig.NC,
		Origin:        daemonConfig.Origin,
		Appversion:    daemonConfig.Appversion,
		DaemonVersion: walletVersion,
	}

	if len(gDaemonClient.Addrs) == 0 {
		xroute, err := bootnode.GetXroute(logger)
		if err != nil {
			panic(err)
		}
		if len(xroute) == 0 {
			panic("bootnode.GetXroute len(xroute) == 0")
		}
		gDaemonClient.Addrs = xroute
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: daemonConfig.SkipVerify,
		},
		ResponseHeaderTimeout: 2 * time.Minute,
		DisableCompression:    true,
		DisableKeepAlives:     false,
		IdleConnTimeout:       2 * time.Minute,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: defaultDialTimeout, KeepAlive: keepAliveInterval}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	gDaemonClient.HTTPClient = &http.Client{
		Transport: transport,
	}
}

// setNextAddr if http do fail,choose another one
func setNextAddr() {
	gDaemonClient.lock.Lock()
	gDaemonClient.AddrIdx++
	if gDaemonClient.AddrIdx >= len(gDaemonClient.Addrs) {
		gDaemonClient.AddrIdx = 0
	}
	gDaemonClient.lock.Unlock()
}

func getAddr() string {
	// gDaemonClient.lock.Lock()
	return gDaemonClient.Addrs[gDaemonClient.AddrIdx]
	// gDaemonClient.lock.Unlock()
}

// CallJSONRPC call  /json_rpc func
// curl -X POST http://127.0.0.1:18081/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"get_block","params":{"height":912345}}' -H 'Content-Type: application/json'
func CallJSONRPC(method string, params interface{}) ([]byte, error) {
	urlPath := ""
	if len(method) >= 4 {
		urlPath = method[4:]
	}

	url := fmt.Sprintf("%s/%s", getAddr(), urlPath)

	requestData := make(map[string]interface{})

	requestData["jsonrpc"] = "2.0"
	requestData["id"] = 1
	requestData["method"] = method
	requestData["params"] = params

	client := gDaemonClient.HTTPClient
	data, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}
	log.Debug("CallJSONRPC", "url", url, "data", string(data))
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("NewRequest: err=%v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("nc", gDaemonClient.NC)
	req.Header.Add("origin", gDaemonClient.Origin)
	req.Header.Add("appversion", gDaemonClient.Appversion)
	req.Header.Add("daemonversion", gDaemonClient.DaemonVersion)

	req = req.WithContext(context.Background())
	resp, err := client.Do(req)
	if err != nil {
		// log.Error("CallJSONRPC client.Do", "err", err)
		setNextAddr()
		return nil, fmt.Errorf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("StatusCode %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %v", err)
	}

	return body, nil
}
