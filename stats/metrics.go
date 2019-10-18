package stats

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// PacketsReceived is a counter of packets received
	PacketsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gateway",
		Name:      "packets_received",
		Help:      "Counter of packets received over all gateway connections.",
	}, []string{"t", "op", "shard"})

	// PacketsSent is a counter of packets sent
	PacketsSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gateway",
		Name:      "packets_sent",
		Help:      "Counter of packets sent over all gateway connections.",
	}, []string{"t", "op", "shard"})

	// ShardsAlive is a gauge of the number of shards alive
	ShardsAlive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gateway",
		Name:      "shards_alive",
		Help:      "Number of shards that are online",
	}, []string{"id"})

	// TotalShards is a gauge of the total number of shards
	TotalShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "gateway",
		Name:      "total_shards",
		Help:      "Total number of shards that should be online.",
	})

	// Ping is a summary of shard heartbeat latency
	Ping = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "gateway",
		Name:      "ping",
		Help:      "Latency between heartbeat and acknowledgement (in milliseconds).",
		Objectives: map[float64]float64{
			0.5:  0.05,
			0.9:  0.01,
			0.95: 0.005,
			0.99: 0.001,
		},
	}, []string{"id"})
)

func init() {
	prometheus.MustRegister(PacketsReceived, PacketsSent, ShardsAlive, TotalShards, Ping)
}
