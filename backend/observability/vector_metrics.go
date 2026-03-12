package observability

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

type histogram struct {
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
	mu      sync.Mutex
}

func newHistogram(buckets []float64) *histogram {
	cp := append([]float64(nil), buckets...)
	sort.Float64s(cp)
	return &histogram{
		buckets: cp,
		counts:  make([]uint64, len(cp)),
	}
}

func (h *histogram) observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += v
	h.count++
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
		}
	}
}

func (h *histogram) snapshot() (buckets []float64, counts []uint64, sum float64, count uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	buckets = append([]float64(nil), h.buckets...)
	counts = append([]uint64(nil), h.counts...)
	return buckets, counts, h.sum, h.count
}

type VectorControlMetrics struct {
	mu sync.RWMutex

	vectorSearchDebugTotal     uint64
	vectorDeleteByFilterTotal  uint64
	vectorReindexDocumentTotal uint64
	vectorReindexAllTotal      uint64
	vectorRepairTotal          uint64
	vectorConsistencyErrTotal  uint64

	vectorOrphanCount  float64
	vectorMissingCount float64

	searchDebugDuration *histogram
	healthScanDuration  *histogram
}

func NewVectorControlMetrics() *VectorControlMetrics {
	return &VectorControlMetrics{
		searchDebugDuration: newHistogram([]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30}),
		healthScanDuration:  newHistogram([]float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 60}),
	}
}

var Metrics = NewVectorControlMetrics()

func (m *VectorControlMetrics) IncSearchDebugTotal() {
	m.mu.Lock()
	m.vectorSearchDebugTotal++
	m.mu.Unlock()
}

func (m *VectorControlMetrics) IncDeleteByFilterTotal() {
	m.mu.Lock()
	m.vectorDeleteByFilterTotal++
	m.mu.Unlock()
}

func (m *VectorControlMetrics) IncReindexDocumentTotal() {
	m.mu.Lock()
	m.vectorReindexDocumentTotal++
	m.mu.Unlock()
}

func (m *VectorControlMetrics) IncReindexAllTotal() {
	m.mu.Lock()
	m.vectorReindexAllTotal++
	m.mu.Unlock()
}

func (m *VectorControlMetrics) IncVectorRepairTotal(delta int) {
	if delta <= 0 {
		return
	}
	m.mu.Lock()
	m.vectorRepairTotal += uint64(delta)
	m.mu.Unlock()
}

func (m *VectorControlMetrics) IncConsistencyErrorTotal() {
	m.mu.Lock()
	m.vectorConsistencyErrTotal++
	m.mu.Unlock()
}

func (m *VectorControlMetrics) SetOrphanCount(v int) {
	m.mu.Lock()
	m.vectorOrphanCount = float64(v)
	m.mu.Unlock()
}

func (m *VectorControlMetrics) SetMissingCount(v int) {
	m.mu.Lock()
	m.vectorMissingCount = float64(v)
	m.mu.Unlock()
}

func (m *VectorControlMetrics) ObserveSearchDebugDuration(seconds float64) {
	m.searchDebugDuration.observe(seconds)
}

func (m *VectorControlMetrics) ObserveHealthScanDuration(seconds float64) {
	m.healthScanDuration.observe(seconds)
}

func (m *VectorControlMetrics) RenderPrometheus() string {
	m.mu.RLock()
	searchTotal := m.vectorSearchDebugTotal
	deleteTotal := m.vectorDeleteByFilterTotal
	reindexDocTotal := m.vectorReindexDocumentTotal
	reindexAllTotal := m.vectorReindexAllTotal
	repairTotal := m.vectorRepairTotal
	consistencyErrTotal := m.vectorConsistencyErrTotal
	orphan := m.vectorOrphanCount
	missing := m.vectorMissingCount
	m.mu.RUnlock()

	var b strings.Builder
	writeCounter(&b, "vector_search_debug_total", "Total number of admin search debug requests", searchTotal)
	writeCounter(&b, "vector_delete_by_filter_total", "Total number of admin delete by filter requests", deleteTotal)
	writeCounter(&b, "vector_reindex_document_total", "Total number of admin reindex document requests", reindexDocTotal)
	writeCounter(&b, "vector_reindex_all_total", "Total number of admin reindex all requests", reindexAllTotal)
	writeCounter(&b, "vector_repair_total", "Total number of processed vector repair tasks", repairTotal)
	writeCounter(&b, "vector_consistency_error_total", "Total number of vector consistency scan errors", consistencyErrTotal)

	writeGauge(&b, "vector_orphan_count", "Last observed orphan vector count", orphan)
	writeGauge(&b, "vector_missing_count", "Last observed missing vector count", missing)

	writeHistogram(&b, "vector_search_debug_duration_seconds", "Duration of admin search debug requests in seconds", m.searchDebugDuration)
	writeHistogram(&b, "vector_health_scan_duration_seconds", "Duration of vector health scans in seconds", m.healthScanDuration)
	return b.String()
}

func MetricsHandler(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	return c.SendString(Metrics.RenderPrometheus())
}

func writeCounter(b *strings.Builder, name, help string, value uint64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s %d\n", name, value)
}

func writeGauge(b *strings.Builder, name, help string, value float64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s gauge\n", name)
	fmt.Fprintf(b, "%s %s\n", name, strconv.FormatFloat(value, 'f', -1, 64))
}

func writeHistogram(b *strings.Builder, name, help string, h *histogram) {
	buckets, counts, sum, totalCount := h.snapshot()
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s histogram\n", name)
	var cumulative uint64
	for i, le := range buckets {
		cumulative += counts[i]
		fmt.Fprintf(b, "%s_bucket{le=\"%s\"} %d\n", name, strconv.FormatFloat(le, 'f', -1, 64), cumulative)
	}
	fmt.Fprintf(b, "%s_bucket{le=\"+Inf\"} %d\n", name, totalCount)
	fmt.Fprintf(b, "%s_sum %s\n", name, strconv.FormatFloat(sum, 'f', -1, 64))
	fmt.Fprintf(b, "%s_count %d\n", name, totalCount)
}
