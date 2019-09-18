package ethapi

import (
	"context"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/libs/log"
)

type PublicPrometheusMetricsAPI struct {
	b Backend
}

func NewPublicPrometheusMetricsAPI(b Backend) *PublicPrometheusMetricsAPI {
	return &PublicPrometheusMetricsAPI{
		b: b,
	}
}

func (s *PublicPrometheusMetricsAPI) PrometheusMetrics() string {
	prometheusMetrics := s.b.PrometheusMetrics()

	specGoodTxs, goodTxs, futureTxs := s.b.Stats()
	untreatedTxs := specGoodTxs + goodTxs + futureTxs
	prometheusMetrics += metrics.PrometheusMetricInstance.GenStandardMetric("untreatedTxs", uint64(untreatedTxs))

	header, err := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber)
	if err != nil {
		log.Error("PrometheusMetrics Get HeaderByNumber failed.", "err", err.Error(), "blockNumber", rpc.LatestBlockNumber)
		return prometheusMetrics
	}
	blockHeight := header.Height
	block, err := s.b.BlockByNumber(nil, rpc.BlockNumber(blockHeight))
	if err != nil {
		log.Error("PrometheusMetrics Get BlockByNumber failed.", "err", err.Error(), "blockHeight", blockHeight)
		return prometheusMetrics
	}
	blockResponse := rtypes.NewRPCBlock(block, true, false)
	if rpc.BlockNumber(blockHeight) == rpc.PendingBlockNumber {
		// Pending blocks need to nil out a few fields
		blockResponse.Coinbase = nil
		blockResponse.Hash = nil
	}
	timeLayout := "2006-01-02 03:04:05"
	generateBlockTime := time.Unix(blockResponse.Time.ToInt().Int64(), 0).Format(timeLayout)
	prometheusMetrics += metrics.PrometheusMetricInstance.GenBlockHeightMetric(generateBlockTime, blockHeight)

	nInfo, err := s.b.NetInfo()
	if err != nil {
		log.Error("PrometheusMetrics Get NetInfo failed.", "err", err.Error())
		return err.Error()
	}
	for _, peer := range nInfo.Peers {
		prometheusMetrics += metrics.PrometheusMetricInstance.GenNetInfo(peer.Moniker,
			peer.Type, peer.Version)
	}

	return prometheusMetrics
}