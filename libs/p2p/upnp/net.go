package upnp

import "net"

// Netlist is a list of IP networks.
type Netlist []net.IPNet

var lan4, lan6 Netlist

func init() {
	//ref: https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
	//Last Updated: 2017-07-03
	lan4.Add("0.0.0.0/8")
	lan4.Add("10.0.0.0/8")
	lan4.Add("100.64.0.0/10")
	lan4.Add("127.0.0.0/8")
	lan4.Add("169.254.0.0/16")
	lan4.Add("172.16.0.0/12")
	lan4.Add("192.0.0.0/24")
	//lan4.Add("192.0.0.0/29")
	//lan4.Add("192.0.0.8/32")
	//lan4.Add("192.0.0.9/32")
	//lan4.Add("192.0.0.10/32")
	//lan4.Add("192.0.0.170/32")
	//lan4.Add("192.0.0.171/32")
	lan4.Add("192.0.2.0/24")
	lan4.Add("192.31.196.0/24")
	lan4.Add("192.52.193.0/24")
	lan4.Add("192.88.99.0/24")
	lan4.Add("192.168.0.0/16")
	lan4.Add("192.175.48.0/24")
	lan4.Add("198.18.0.0/15")
	lan4.Add("198.51.100.0/24")
	lan4.Add("203.0.113.0/24")
	lan4.Add("240.0.0.0/4")

	lan6.Add("fe80::/10") // Link-Local
	lan6.Add("fc00::/7")  // Unique-Local
}

// Add parses a CIDR mask and appends it to the list. It panics for invalid masks and is
// intended to be used for setting up static lists.
func (l *Netlist) Add(cidr string) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	*l = append(*l, *n)
}

// Contains reports whether the given IP is contained in the list.
func (l *Netlist) Contains(ip net.IP) bool {
	if l == nil {
		return false
	}
	for _, net := range *l {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

// IsLAN reports whether an IP is a local network address.
// IANA IPv4 Special-Purpose Address Registry, not only local network address
func IsLAN(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return lan4.Contains(v4)
	}
	return lan6.Contains(ip)
}
