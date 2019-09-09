//go test -gcflags=-l -v

package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	. "github.com/bouk/monkey"
	"github.com/lianxiangcloud/linkchain/libs/log"
	cfg "github.com/lianxiangcloud/linkchain/wallet/config"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/h2non/gock.v1"
)

func TestCallJSONRPC(t *testing.T) {
	defer gock.Off()
	config := cfg.DefaultConfig()
	config.Daemon.PeerRPC = "http://127.0.0.1:15000"

	InitDaemonClient(config.Daemon)
	log.ParseLogLevel("*:error", log.Root(), "info")

	host := "http://127.0.0.1:15000/blockNumber"
	matchType := "application/json"
	bodyOK := `{"jsonrpc":"2.0","id":"0","method":"eth_blockNumber","params":[]}`
	replyOK := 200
	replyServerErr := 500
	replyNotFound := 404
	bodyReturnOK := `{"jsonrpc":"2.0","id":"0","result":"0xc0b"}`

	Convey("test daemon.CallJSONRPC", t, func() {
		Convey("http status 200", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyOK).
				BodyString(bodyReturnOK)

			gock.InterceptClient(gDaemonClient.HttpClient)

			p := make([]interface{}, 0)
			body, err := CallJSONRPC("eth_blockNumber", p)
			log.Debug("TestCallJSONRPC", "body", string(body), "err", err)
			//several So assert
			So(strings.Compare(string(body), bodyReturnOK) == 0 && err == nil, ShouldBeTrue)
		})

		Convey("http status 500", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyServerErr).
				BodyString(bodyReturnOK)

			gock.InterceptClient(gDaemonClient.HttpClient)

			p := make([]interface{}, 0)
			_, err := CallJSONRPC("eth_blockNumber", p)
			So(err != nil, ShouldBeTrue)
		})
		Convey("http status 404", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyNotFound).
				BodyString(bodyReturnOK)

			gock.InterceptClient(gDaemonClient.HttpClient)

			p := make([]interface{}, 0)
			_, err := CallJSONRPC("eth_blockNumber", p)
			So(err != nil, ShouldBeTrue)
		})
		Convey("client.Do err", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyOK).
				BodyString(bodyReturnOK)

			client := gDaemonClient.HttpClient

			guard := PatchInstanceMethod(reflect.TypeOf(client), "Do", func(*http.Client, *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("client do err")
			})
			defer guard.Unpatch()

			gock.InterceptClient(client)

			p := make([]interface{}, 0)
			_, err := CallJSONRPC("eth_blockNumber", p)
			So(err != nil, ShouldBeTrue)
		})
		Convey("ioutil.ReadAll err", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyOK).
				BodyString(bodyReturnOK)

			client := gDaemonClient.HttpClient

			guard := Patch(ioutil.ReadAll, func(r io.Reader) ([]byte, error) {
				return nil, fmt.Errorf("ioutil.ReadAll err")
			})
			defer guard.Unpatch()

			gock.InterceptClient(client)

			p := make([]interface{}, 0)
			_, err := CallJSONRPC("eth_blockNumber", p)
			So(err != nil, ShouldBeTrue)
		})
		Convey("http.NewRequest err", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyOK).
				BodyString(bodyReturnOK)

			client := gDaemonClient.HttpClient

			guard := Patch(http.NewRequest, func(method, url string, body io.Reader) (*http.Request, error) {
				return nil, fmt.Errorf("http.NewRequest err")
			})
			defer guard.Unpatch()

			gock.InterceptClient(client)

			p := make([]interface{}, 0)
			_, err := CallJSONRPC("eth_blockNumber", p)
			So(err != nil, ShouldBeTrue)
		})

		Convey("json.Marshal err", func() {
			gock.New(host).
				// Post("").
				MatchType(matchType).
				BodyString(bodyOK).
				Reply(replyOK).
				BodyString(bodyReturnOK)

			client := gDaemonClient.HttpClient

			guard := Patch(json.Marshal, func(v interface{}) ([]byte, error) {
				return nil, fmt.Errorf("json.Marshal err")
			})
			defer guard.Unpatch()

			gock.InterceptClient(client)

			p := make([]interface{}, 0)
			_, err := CallJSONRPC("eth_blockNumber", p)
			So(err != nil, ShouldBeTrue)
		})
	})
}
