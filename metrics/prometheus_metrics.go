package metrics

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
)

type prometheusMetric struct {
	mtx sync.Mutex

	minute int

	cfg    *config.Config
	logger log.Logger
	pubkey crypto.PubKey

	// common data
	hostname     string
	httpEndpoint string
	roleType     types.NodeType

	metrics   string
	cpMetrics string

	currentBlockProposerPubkey crypto.PubKey
}

var PrometheusMetricInstance *prometheusMetric

func init() {
	PrometheusMetricInstance = &prometheusMetric{
		minute: -1,
	}
	go PrometheusMetricInstance.runMetricsCleaner()
}

func (p *prometheusMetric) Init(cfg *config.Config, pubkey crypto.PubKey, logger log.Logger) {
	p.cfg = cfg
	p.logger = logger
	p.pubkey = pubkey

	err := p.initCommon()
	if err != nil {
		p.logger.Error("prometheusMetric init failed.", "err", err.Error())
	}
}

func (p *prometheusMetric) initCommon() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	p.hostname = hostname
	p.httpEndpoint = p.cfg.RPC.HTTPEndpoint

	return nil
}

func (p *prometheusMetric) SetRole(roleType types.NodeType) {
	p.roleType = roleType
}

func (p *prometheusMetric) GetRole() types.NodeType {
	return p.roleType
}

func (p *prometheusMetric) runMetricsCleaner() {
	for {
		currentMinute := time.Now().Minute()
		if p.minute != currentMinute {
			p.minute = currentMinute
			p.cleanMetrics()
		}
		time.Sleep(time.Second)
	}
}

func (p *prometheusMetric) cleanMetrics() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.cpMetrics = p.metrics
	p.metrics = ""
}

func (p *prometheusMetric) AddMetrics(metrics string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.metrics += metrics
}

func (p *prometheusMetric) GetMetrics() string {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.cpMetrics
}

func (p *prometheusMetric) SetCurrentProposerPubkey(pubkey crypto.PubKey) {
	p.currentBlockProposerPubkey = pubkey
}

func (p *prometheusMetric) ProposerPubkeyEquals() bool {
	return p.pubkey.Equals(p.currentBlockProposerPubkey)
}

func (p *prometheusMetric) GenStandardMetric(metricName string, val uint64) string {
	return fmt.Sprintf("linkchain_%s{hostname=\"%s\",role=\"%d\",ip_port=\"%s\"} %d\n",
		metricName, p.hostname, p.roleType, p.httpEndpoint, val)
}

func (p *prometheusMetric) GenBlockHeightMetric(generateBlockTime string, blockHeight uint64) string {
	return fmt.Sprintf("linkchain_BlockHeight{hostname=\"%s\",role=\"%d\",ip_port=\"%s\",generate_block_time=\"%s\"} %d\n",
		p.hostname, p.roleType, p.httpEndpoint, generateBlockTime, blockHeight)
}

func (p *prometheusMetric) GenBlockValidatorsListMetric(validatorsListStr string, blockHeight uint64) string {
	return fmt.Sprintf("link_BlockValidatorsList{hostname=\"%s\",role=\"%d\",ip_port=\"%s\",validators_list=\"%s\"} %d\n",
		p.hostname, p.roleType, p.httpEndpoint, validatorsListStr, blockHeight)
}

func (p *prometheusMetric) GenNetInfo(peerName string, peerRole types.NodeType, peerVersion string) string {
	return fmt.Sprintf("link_NetInfo{hostname=\"%s\",role=\"%d\",ip_port=\"%s\",peerName=\"%s\",peerRole=\"%s\",peerVersion=\"%s\"} 0\n",
		p.hostname, p.roleType, p.httpEndpoint, peerName, peerRole, peerVersion)
}