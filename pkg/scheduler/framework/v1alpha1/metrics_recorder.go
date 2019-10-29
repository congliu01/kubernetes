package v1alpha1

import (
	"time"

	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/kubernetes/pkg/scheduler/metrics"
)

type MetricsRecorder interface {
	observeExtensionPointDurationAsync(extensionPoint string, status *Status, value float64)
	observePluginDurationAsync(extensionPoint, pluginName string, status *Status, value float64)
}

func NewNoopMetricsRecorder() *noopMetricsRecorder {
	return &noopMetricsRecorder{}
}

type noopMetricsRecorder struct{}

func (*noopMetricsRecorder) observeExtensionPointDurationAsync(extensionPoint string, status *Status, value float64) {
}
func (*noopMetricsRecorder) observePluginDurationAsync(extensionPoint, pluginName string, status *Status, value float64) {
}

// frameworkMetric is the data structure passed in the buffer channel between the main framework thread
// and the metricsRecorder goroutine.
type frameworkMetric struct {
	metric      *k8smetrics.HistogramVec
	labelValues []string
	value       float64
}

// metricRecorder records framework metrics in a separate goroutine to avoid overhead in the critical path.
type metricsRecorder struct {
	// bufferCh is a channel that serves as a metrics buffer before the metricsRecorder goroutine reports it.
	bufferCh chan *frameworkMetric
	// if bufferSize is reached, incoming metrics will be discarded.
	bufferSize int

	stopCh      chan struct{}
	isStoppedCh chan struct{}
}

func NewMetricsRecorder(bufferSize int) *metricsRecorder {
	recorder := &metricsRecorder{
		bufferCh:    make(chan *frameworkMetric, bufferSize),
		bufferSize:  bufferSize,
		stopCh:      make(chan struct{}),
		isStoppedCh: make(chan struct{}),
	}
	go recorder.run()
	return recorder
}

func (r *metricsRecorder) observeExtensionPointDurationAsync(extensionPoint string, status *Status, value float64) {
	newMetric := &frameworkMetric{
		metric:      metrics.FrameworkExtensionPointDuration,
		labelValues: []string{extensionPoint, status.Code().String()},
		value:       value,
	}
	select {
	case r.bufferCh <- newMetric:
	default:
	}
}

func (r *metricsRecorder) observePluginDurationAsync(extensionPoint, pluginName string, status *Status, value float64) {
	newMetric := &frameworkMetric{
		metric:      metrics.PluginExecutionDuration,
		labelValues: []string{pluginName, extensionPoint, status.Code().String()},
		value:       value,
	}
	select {
	case r.bufferCh <- newMetric:
	default:
	}
}

// run records metrics into Prometheus every second.
func (r *metricsRecorder) run() {
	for {
		select {
		case <-r.stopCh:
			close(r.isStoppedCh)
			return
		default:
		}
		r.recordMetrics()
		time.Sleep(time.Second)
	}
}

// recordMetrics tries to clean up the bufferCh by reading at most bufferSize metrics.
// This is used for testing to make sure metrics are recorded.
func (r *metricsRecorder) recordMetrics() {
	for i := 0; i < r.bufferSize; i++ {
		select {
		case m := <-r.bufferCh:
			m.metric.WithLabelValues(m.labelValues...).Observe(m.value)
		default:
			return
		}
	}
}
