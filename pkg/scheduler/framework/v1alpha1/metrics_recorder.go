package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/wait"
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/kubernetes/pkg/scheduler/metrics"
	"sync"
	"time"
)

// frameworkMetric is the data structure passed in the buffer channel between the main framework thread
// and the metricsRecorder goroutine.
type frameworkMetric struct {
	metric *k8smetrics.HistogramVec
	labelValues []string
	value float64
}

// metricRecorder records framework metrics in a separate goroutine to avoid overhead in the critical path.
type metricsRecorder struct {
	// bufferCh is a channel that serves as a metrics buffer before the metricsRecorder goroutine reports it.
	bufferCh   *chan *frameworkMetric
	batchCh *chan *frameworkMetric
	// if bufferSize is reached, incoming metrics will be discarded.
	batchSize int

	mu sync.RWMutex


	// stopCh can be used to stop the metricsRecorder goroutine.
	stopCh     chan struct{}
	//
	stoppedCh  chan struct{}
}

func newMetricsRecorder(stopCh chan struct{}, bufferSize int) *metricsRecorder {
	//fmt.Println("Creating recorder... ")
	bufferCh := make(chan *frameworkMetric, bufferSize)
	batchCh := make(chan *frameworkMetric, bufferSize)
	recorder := &metricsRecorder{
		bufferCh:   &bufferCh,
		batchCh: &batchCh,
		stopCh:     stopCh,
		stoppedCh:  make(chan struct{}),
	}
	go wait.Until(recorder.tryCleanUpBuffer, 2*time.Second, recorder.stopCh)
	//go func() {recorder.run()}()
	return recorder
}

func (r *metricsRecorder) observeExtensionPointDurationAsync(extensionPoint string, status *Status, value float64) {
	//fmt.Println("Recording extension point metrics: ")

	newMetric := &frameworkMetric{
		metric: metrics.FrameworkExtensionPointDuration,
		labelValues:[]string{extensionPoint, status.Code().String()},
		value:value,
	}
	select {
	case *r.bufferCh <- newMetric:
		//fmt.Println("Recording extension point metrics DONE: ", newMetric)

	default:
	}
}

func (r *metricsRecorder) observePluginDurationAsync(pluginName, extensionPoint string, status *Status, value float64) {
	//fmt.Println("Recording plugin metrics: ")
	newMetric := &frameworkMetric{
		metric: metrics.PluginExecutionDuration,
		labelValues:[]string{pluginName, extensionPoint, status.Code().String()},
		value:value,
	}
	select {
	case *r.bufferCh <- newMetric:
		//fmt.Println("Recording plugins metrics DONE: ", newMetric)

	default:
	}
}

// tryCleanUpBuffer tries to clean up the bufferCh by reading at most bufferSize metrics.
// This is used for testing to make sure metrics are recorded.
func  (r *metricsRecorder) tryCleanUpBuffer() {
	//close(r.stopCh)
	//<-r.stoppedCh
	// switch channels
	r.mu.Lock()
	r.batchCh = r.bufferCh
	r.bufferCh = r.batchCh
	r.mu.Unlock()
	for {
		//fmt.Println("Cleaning up metrics: ", i)
		select {
		case m := <- *r.batchCh:
			//fmt.Println("Got one metric: ", m)
			m.metric.WithLabelValues(m.labelValues...).Observe(m.value)
			default:
				return
		}
	}
}
