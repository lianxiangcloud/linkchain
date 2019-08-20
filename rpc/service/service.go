package service

import (
	"fmt"
	"net"
	"strings"
 	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/rpc/ethapi"
	"golang.org/x/time/rate"
)

type Service struct {
	logger  log.Logger
	conf    *config.RPCConfig
	ctx     *Context
	backend *ApiBackend

	ipcListener   net.Listener // IPC RPC listener socket to serve API requests
	ipcHandler    *rpc.Server  // IPC RPC request handler to process the API requests
	httpWhitelist []string     // HTTP RPC modules to allow through this endpoint
	httpListener  net.Listener // HTTP RPC listener socket to server API requests
	httpHandler   *rpc.Server  // HTTP RPC request handler to process the API requests
	wsListener    net.Listener // Websocket RPC listener socket to server API requests
	wsHandler     *rpc.Server  // Websocket RPC request handler to process the API requests

	apis     []rpc.API
	pubsub   *PubsubApi
	bloom    *BloomService
	evmLimit *rate.Limiter
}

// New new rpc service
func New(conf *config.RPCConfig, ctx *Context) *Service {
	s := &Service{
		conf:     conf,
		ctx:      ctx,
		logger:   ctx.logger,
		evmLimit: rate.NewLimiter(rate.Every(conf.EVMInterval), conf.EVMMax),
	}
	backend := NewApiBackend(s)
	s.backend = backend
	s.apis = ethapi.GetAPIs(s.backend)
	s.bloom = NewBloomService(s)

	s.pubsub = &PubsubApi{
		s:     s,
		txMgr: ctx.txService,
	}
	api := rpc.API{
		Namespace: "lk",
		Version:   "1.0",
		Service:   s.pubsub,
		Public:    true,
	}
	s.apis = append(s.apis, api)
	return s
}

// Start rpc service
func (s *Service) Start() error {
	if err := s.bloom.Start(); err != nil {
		return err
	}

	if err := s.startIPC(); err != nil {
		return err
	}

	if err := s.startHTTP(); err != nil {
		s.stopIPC()
		return err
	}

	if err := s.startWS(); err != nil {
		s.stopHTTP()
		s.stopIPC()
		return err
	}
	return nil
}

// Stop rpc service
func (s *Service) Stop() {
	s.stopWS()
	s.stopHTTP()
	s.stopIPC()
	s.bloom.Stop()
}

func (s *Service) context() *Context {
	return s.ctx
}

func (s *Service) apiBackend() *ApiBackend {
	return s.backend
}

func (s *Service) setApi(apiBackend ethapi.Backend) {
	s.apis = ethapi.GetAPIs(apiBackend)
}

// startIPC initializes and starts the IPC RPC endpoint.
func (s *Service) startIPC() error {
	if s.conf.IpcEndpoint == "" {
		return nil // IPC disabled.
	}

	listener, handler, err := rpc.StartIPCEndpoint(s.conf.IPCFile(), s.apis)
	if err != nil {
		return err
	}

	s.ipcListener = listener
	s.ipcHandler = handler
	s.logger.Info("IPC endpoint opened", "url", s.conf.IPCFile())
	return nil
}

// stopIPC terminates the IPC RPC endpoint.
func (s *Service) stopIPC() {
	if s.ipcListener != nil {
		s.ipcListener.Close()
		s.ipcListener = nil

		s.logger.Info("IPC endpoint closed", "endpoint", s.conf.IPCFile())
	}
	if s.ipcHandler != nil {
		s.ipcHandler.Stop()
		s.ipcHandler = nil
	}
}

// startHTTP initializes and starts the HTTP RPC endpoint.
func (s *Service) startHTTP() error {
	// Short circuit if the HTTP endpoint isn't being exposed
	if s.conf.HTTPEndpoint == "" {
		return nil
	}

	listener, handler, err := rpc.StartHTTPEndpoint(s.conf.HTTPEndpoint, s.apis, s.conf.HTTPModules, s.conf.HTTPCores, s.conf.VHosts)
	if err != nil {
		return err
	}
	s.logger.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", s.conf.HTTPEndpoint),
		"cors", strings.Join(s.conf.HTTPCores, ","), "vhosts", strings.Join(s.conf.VHosts, ","), "modules", strings.Join(s.conf.HTTPModules, ","))

	// All listeners booted successfully
	s.httpListener = listener
	s.httpHandler = handler
	return nil
}

// stopHTTP terminates the HTTP RPC endpoint.
func (s *Service) stopHTTP() {
	if s.httpListener != nil {
		s.httpListener.Close()
		s.httpListener = nil

		s.logger.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", s.conf.HTTPEndpoint))
	}
	if s.httpHandler != nil {
		s.httpHandler.Stop()
		s.httpHandler = nil
	}
}

// startWS initializes and starts the websocket RPC endpoint.
func (s *Service) startWS() error {
	// Short circuit if the WS endpoint isn't being exposed
	if s.conf.WSEndpoint == "" {
		return nil
	}

	listener, handler, err := rpc.StartWSEndpoint(s.conf.WSEndpoint, s.apis, s.conf.WSModules, s.conf.WSOrigins, s.conf.WSExposeAll)
	if err != nil {
		return err
	}
	s.logger.Info("WebSocket endpoint opened", "url", fmt.Sprintf("ws://%s", listener.Addr()))

	// All listeners booted successfully
	s.wsListener = listener
	s.wsHandler = handler
	return nil
}

// stopWS terminates the websocket RPC endpoint.
func (s *Service) stopWS() {
	if s.wsListener != nil {
		s.wsListener.Close()
		s.wsListener = nil

		s.logger.Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s", s.conf.WSEndpoint))
	}
	if s.wsHandler != nil {
		s.wsHandler.Stop()
		s.wsHandler = nil
	}
}
