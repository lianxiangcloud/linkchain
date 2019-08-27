package bootcli

import "net"

// Endpoint network endpoint
type Endpoint struct {
	IP   []string       `json:"ip"`
	Port map[string]int `json:"port"` //key:protocol(udp or tcp or http)
}

// NewLocalEndpoint create network endpoint with local IPv4 address
func NewLocalEndpoint() (*Endpoint, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var ip []string
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.IsLoopback() {
			continue
		}
		if ipnet.IP.To4() != nil {
			ip = append(ip, ipnet.IP.String())
		}
	}

	return &Endpoint{
		IP:   ip,
		Port: make(map[string]int),
	}, nil
}
