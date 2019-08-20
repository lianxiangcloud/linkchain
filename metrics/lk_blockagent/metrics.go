package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"strconv"
	"sync"

	"github.com/pkg/errors"
)

const (
	succ                                 = 0
	refreshCenterDataTimeIntervalMinutes = 10
)

type endPoint struct {
	IP   []string       `json:"ip"`
	Port map[string]int `json:"port"`
}

type node struct {
	Type     int      `json:"type,omitempty"`
	HostName string   `json:"hostname,omitempty"`
	PubKey   string   `json:"pubkey,omitempty"`
	EndPoint endPoint `json:"endpoint"`
}

type bootSvrEndPoints struct {
	Code  int    `json:"code,omitempty"`
	Nodes []node `json:"nodes"`
}

func runLKBlockAgent(configs *lkBlockAgentConfigs) {
	m := &metrics{
		configs:           configs,
		forEachtNodeTypes: make([]int, 0),
	}
	log.Info("Start getCommonMetrics.")

	cMetrics := &commonMetrics{}
	err := m.getCommonMetrics(cMetrics)
	if err != nil {
		log.Error("runLKBlockAgent get common metrifcs failed.", "err", err.Error())
		return
	}
	m.cMetrics = cMetrics

	err = m.getConfigCenterData()
	if err != nil {
		log.Error("getConfigCenterData exec failed.", "err", err.Error())
		return
	}

	go func() {
		for {
			m.globalMu.Lock()
			defer m.globalMu.Unlock()

			log.Info("Start getMetrics.")
			prometheusMetrics := m.getMetrics()
			log.Info("start send metrics to collector", "metrics", prometheusMetrics)
			m.sendMetricsToMetricsCollector(prometheusMetrics)
			time.Sleep(time.Minute)
		}
	}()

	go func() {
		for {
			m.globalMu.Lock()
			defer m.globalMu.Unlock()

			time.Sleep(refreshCenterDataTimeIntervalMinutes * time.Minute)
			log.Info("Start to update getConfigCenterData")
			err = m.getConfigCenterData()
			if err != nil {
				log.Error("getConfigCenterData exec failed.", "err", err.Error())
			}
		}
	}()

}

type metrics struct {
	configs           *lkBlockAgentConfigs
	forEachtNodeTypes []int
	cMetrics          *commonMetrics

	globalMu          sync.Mutex
}

type commonMetrics struct {
	hostname            string
	extIpaddr           string
	bootSvrEndPointsObj bootSvrEndPoints
}

func (m *metrics) getCommonMetrics(cMetrics *commonMetrics) error {
	// get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	cMetrics.hostname = hostname

	// get ext ip address
	addrs, err := net.InterfaceAddrs()
	for _, address := range addrs {
		// Check ip address. Can
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				if ipnet.IP.To4()[0] != 10 &&
					ipnet.IP.To4()[0] != 172 &&
					ipnet.IP.To4()[0] != 192 {
					cMetrics.extIpaddr = ipnet.IP.String()
				}
			}
		}
	}

	if len(cMetrics.hostname) == 0 || len(cMetrics.extIpaddr) == 0 {
		log.Error("Get common metrics faield.", "hostname", cMetrics.hostname,
			"ext_ipaddr", cMetrics.extIpaddr)
		return errors.New("Get common metrics failed. hostname or ext_ipaddr is nil")
	}
	log.Info("Get common metrics success.", "hostname", cMetrics.hostname,
		"ext_ipaddr", cMetrics.extIpaddr)

	if len(m.configs.ForeachNodeType) == 0 {
		log.Error("ForeachNodeType is empty.")
		return errors.New("ForeachNodeType is empty.")
	}
	forEachtNodeTypesStr := strings.Split(m.configs.ForeachNodeType, ",")
	for _, nodeTypeStr := range forEachtNodeTypesStr {
		nodeType, err := strconv.Atoi(nodeTypeStr)
		if err != nil {
			log.Error("split ForeachNodeType failed.", "err", err.Error())
		}
		m.forEachtNodeTypes = append(m.forEachtNodeTypes, nodeType)
	}

	if len(m.configs.BootNodeEndPointUrl) == 0 {
		log.Error("BootNodeEndPointUrl is empty.")
		return errors.New("BootNodeEndPointUrl is empty.")
	}

	return nil
}


func (m *metrics) isNodeTypeNeedForeach(nodeType int) bool {
	for _, foreachNodeType := range m.forEachtNodeTypes {
		if foreachNodeType == nodeType {
			return true
		}
	}
	return false
}

func (m *metrics) getMetrics() string {
	prometheusMetrics := ""
	// 1. get local machine cpu and memory metrics
	prometheusMetrics += m.getCpuMemoryMetrics()

	// 3. get all metrics
	for _, xnodeDataObj := range m.cMetrics.bootSvrEndPointsObj.Nodes {
		if !m.isNodeTypeNeedForeach(xnodeDataObj.Type) {
			continue
		}
		//if xnodeDataObj.HostName != m.cMetrics.hostname {
		//	continue
		//}
		for _, endPointIp := range xnodeDataObj.EndPoint.IP {
			if strings.Index(endPointIp, m.cMetrics.extIpaddr) == -1 {
				continue
			}
			url := fmt.Sprintf("http://%s:%d", endPointIp, xnodeDataObj.EndPoint.Port["http"])
			post := []byte("{\"jsonrpc\":\"2.0\",\"method\":\"eth_prometheusMetrics\",\"params\":[],\"id\":1}")
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(post))
			if err != nil {
				log.Error("new request failed.", "err", err.Error())
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Error("client.Do failed", "err", err.Error())
				continue
			}
			defer resp.Body.Close()

			metricsBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Error("ioutil.ReadAll failed", "err", err.Error())
				continue
			}
			prometheusMetrics += string(metricsBytes)
		}
	}

	return prometheusMetrics
}

func (m *metrics) genCpuMetric(cpuInfo string) string {
	return fmt.Sprintf("linkchain_Cpuinfo{hostname=\"%s\",cpu_info=\"%s\"} 0",
		m.cMetrics.hostname, cpuInfo)
}

func (m *metrics) genMemoryMetric(memInfo string) string {
	return fmt.Sprintf("linkchain_Cpuinfo{hostname=\"%s\",mem_info=\"%s\"} 0",
		m.cMetrics.hostname, memInfo)
}

func (m *metrics) getCpuMemoryMetrics() string {
	cmd := exec.Command("bash", "-c", "top n 1 b i | grep Cpu")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("command exec failed.", "err", err.Error())
		return ""
	}
	err = cmd.Start()
	if err != nil {
		log.Error("cpu info command exec failed.", "err", err.Error())
		return ""
	}
	cpuInfo, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Error("read cpuinfo failed.", "err", err.Error(), "info", string(cpuInfo))
		return ""
	}
	if err := cmd.Wait(); err != nil {
		log.Error("cmd wait failed.", "err", err.Error())
		return ""
	}
	cpuMetric := m.genCpuMetric(string(cpuInfo))

	cmd = exec.Command("bash", "-c", "top n 1 b i | grep Mem")
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		log.Error("command exec failed.", "err", err.Error())
		return ""
	}
	err = cmd.Start()
	if err != nil {
		log.Error("memory info command exec failed.", "err", err.Error())
		return ""
	}
	memInfo, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Info("read memInfo failed.", "err", err.Error(), "info", string(memInfo))
		return ""
	}
	if err := cmd.Wait(); err != nil {
		log.Error("cmd wait failed.", "err", err.Error())
		return ""
	}
	memMetric := m.genMemoryMetric(string(memInfo))

	return cpuMetric + memMetric
}

func (m *metrics) getConfigCenterData() error {
	url := m.configs.BootNodeEndPointUrl
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(nil))
	if err != nil {
		log.Error("new request failed.", "err", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("client.Do failed", "err", err.Error())
		return err
	}
	defer resp.Body.Close()

	endpointBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("ioutil.ReadAll failed", "err", err.Error())
		return err
	}
	err = json.Unmarshal(endpointBytes, &m.cMetrics.bootSvrEndPointsObj)
	if err != nil {
		log.Error("json unmarshal bootSvrEndPoints failed.", "err", err.Error())
		return err
	}
	if m.cMetrics.bootSvrEndPointsObj.Code != succ {
		log.Error("request bootSvrEndPoints failed.", "code", m.cMetrics.bootSvrEndPointsObj.Code)
		return errors.New("request bootSvrEndPoints failed.")
	}

	return nil
}

func (m *metrics) sendMetricsToMetricsCollector(promethuesMetrics string) {
	url := m.configs.MetricsCollectorUrl
	post := []byte(promethuesMetrics)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(post))
	if err != nil {
		log.Error("new request failed.", "err", err.Error())
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("client.Do failed", "err", err.Error())
	}
	defer resp.Body.Close()
}
