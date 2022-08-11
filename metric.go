package grpc

import (
	"fmt"
	"net/http"

	"github.com/aluka-7/metric"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	serverNamespace = "rpc_server"
	clientNamespace = "rpc_client"
)

var (
	metricServerReqDur = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: serverNamespace,
		Subsystem: "requests",
		Name:      "duration_ms",
		Help:      "rpc server requests duration(ms).",
		Labels:    []string{"method", "caller"},
		Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000},
	})
	metricServerReqCodeTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: serverNamespace,
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "rpc server requests code count.",
		Labels:    []string{"method", "caller", "code"},
	})
	metricClientReqDur = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: clientNamespace,
		Subsystem: "requests",
		Name:      "duration_ms",
		Help:      "rpc client requests duration(ms).",
		Labels:    []string{"method"},
		Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000},
	})
	metricClientReqCodeTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: clientNamespace,
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "rpc client requests code count.",
		Labels:    []string{"method", "code"},
	})
)

// 基于prometheus实现指标收集功能
func metrics() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		h := promhttp.Handler()
		h.ServeHTTP(w, r)
	})
	fmt.Println("RPC即将开启metrics服务,访问地址 http://ip:7070")
	if err := http.ListenAndServe(":7070", nil); err != nil {
		fmt.Printf("RPC开启metrics服务错误:%+v\n", err)
	}
}
