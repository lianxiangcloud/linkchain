package upnp

import (
	"net"
	"time"

	upnpc "github.com/NebulousLabs/go-upnp"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p/netutil"
)

type ExtAddr struct {
	IP   string
	Port uint16
}

type Proxy struct {
	logger      log.Logger
	Port        uint16
	Name        string
	ExtAddrChan chan ExtAddr
	igd         *upnpc.IGD
	StopChan    chan bool
}

func NewProxy(port uint16, name string, logger log.Logger) *Proxy {
	b := new(Proxy)
	b.logger = logger
	b.Port = port
	b.Name = name
	b.ExtAddrChan = make(chan ExtAddr)
	return b
}

func (b *Proxy) Start() {
	go b.loop()
}

func (b *Proxy) loop() {
	defer close(b.ExtAddrChan)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		b.logger.Warn("net.InterfaceAddrs fail", "err", err)
		return
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if !netutil.IsLAN(ipnet.IP) {
				b.logger.Info("loop", "public address", ipnet.IP)
				return
			}
		}
	}

	b.igd, err = upnpc.Discover()
	if err != nil {
		b.logger.Info("discover failed", "err", err)
		return
	}

	extIP, err := b.igd.ExternalIP()
	if err != nil {
		b.logger.Info("get external IP failed", "err", err)
		return
	} else {
		if netutil.IsLAN(net.ParseIP(extIP)) {
			b.logger.Info("loop", "external IP is a local IP", extIP)
			return
		}
	}

	b.addPortMapping()

}

func (b *Proxy) forward(port uint16) {
	err := b.igd.Forward(port, b.Name)
	if err == nil {
		extIP, err := b.igd.ExternalIP()
		if err == nil {
			if len(extIP) == 0 || netutil.IsLAN(net.ParseIP(extIP)) {
				b.logger.Info("external IP is a local IP or is nil", "extIP", extIP)
				return
			} else {
				tick := time.NewTicker(time.Duration(30) * time.Second)
				select {
				case b.ExtAddrChan <- ExtAddr{extIP, port}:
					b.logger.Info("add port mapping success", "port", port)
					break
				case <-tick.C:
					b.logger.Info("forward tick Timeout")
					break
				}
				tick.Stop()
			}
		} else {
			b.logger.Info("get external IP failed", "err", err)
		}
	} else {
		b.logger.Info("add port mapping failed", "err", err)
	}
}

func (b *Proxy) addPortMapping() {
	internalPort, internalAddr, enableFlag, description, durration, err := b.igd.GetSpecificPortMappingEntry("", b.Port)
	b.logger.Debug("addPortMapping", "internalPort", internalPort, "internalAddr", internalAddr, "enableFlag", enableFlag,
		"description", description, "durration", durration)
	if err != nil {
		b.logger.Info("addPortMapping", "GetSpecificPortMappingEntry failed,start forward port", "port", b.Port) //never mapped
		b.forward(b.Port)
		return
	} else { //has been mapped
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			b.logger.Info("InterfaceAddrs get failed", "err", err)
			return
		}

		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					if ipnet.IP.String() == internalAddr { //has been mapped by myself
						b.logger.Debug("addPortMapping", "internalAddr", internalAddr, "has been Previously mappedmapped by myself in port", b.Port)
						b.igd.Clear(uint16(b.Port))
						b.forward(uint16(b.Port))
						return
					}
				}
			}
		}
		//chose another port to map
		startport := b.Port
		for i := 0; i < 1000 && startport < 65535; i++ {
			startport = startport + 1
			internalPort, internalAddr, enableFlag, description, durration, err := b.igd.GetSpecificPortMappingEntry("", uint16(startport))
			b.logger.Debug("addPortMapping", "internalPort", internalPort, "internalAddr", internalAddr, "enableFlag", enableFlag,
				"description", description, "durration", durration)
			if err != nil {
				b.logger.Info("addPortMapping", "start forward port", startport) //never mapped
				b.forward(startport)
				return
			} else {
				continue
			}
		}

	}
}
