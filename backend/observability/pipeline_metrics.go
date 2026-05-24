package observability

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/khiemnd777/legal_api/domain"
)

func RenderPipelineHealthPrometheus(health domain.PipelineHealth) string {
	var b strings.Builder
	w := newPrometheusGaugeWriter(&b)
	w.gauge("pipeline_metrics_up", "Whether pipeline metrics were collected successfully", 1)
	w.gauge("pipeline_generated_at_seconds", "Unix timestamp for the pipeline health snapshot", float64(health.GeneratedAt.Unix()))
	w.labeledGauge("pipeline_health_status", "Current pipeline health severity as one-hot gauges", map[string]string{"severity": "ok"}, boolGauge(health.Severity == "ok"))
	w.labeledGauge("pipeline_health_status", "Current pipeline health severity as one-hot gauges", map[string]string{"severity": "degraded"}, boolGauge(health.Severity == "degraded"))
	w.labeledGauge("pipeline_health_status", "Current pipeline health severity as one-hot gauges", map[string]string{"severity": "critical"}, boolGauge(health.Severity == "critical"))

	w.gauge("pipeline_uploads_total_count", "Total uploads currently tracked by the intake pipeline", float64(health.Summary.TotalUploads))
	w.gauge("pipeline_processing_uploads", "Uploads currently in processing states", float64(health.Summary.ProcessingUploads))
	w.gauge("pipeline_review_uploads", "Uploads currently waiting for review", float64(health.Summary.ReviewUploads))
	w.gauge("pipeline_failed_uploads", "Uploads currently in failed states", float64(health.Summary.FailedUploads))
	w.gauge("pipeline_published_uploads", "Uploads currently published or ready", float64(health.Summary.PublishedUploads))
	w.gauge("pipeline_stale_uploads", "Uploads stuck beyond the stale threshold", float64(health.Summary.StaleUploads))
	w.gauge("pipeline_recent_issues", "Recent pipeline issues in the health window", float64(health.Summary.RecentIssues))
	w.gauge("pipeline_active_ingest_jobs", "Ingest jobs currently active or queued", float64(health.Summary.ActiveJobs))
	w.gauge("pipeline_failed_ingest_jobs", "Ingest jobs currently failed", float64(health.Summary.FailedJobs))

	for _, item := range health.UploadStatusCount {
		w.labeledGauge("pipeline_upload_status_count", "Uploads by current status", map[string]string{"status": item.Status}, float64(item.Count))
	}
	for _, item := range health.JobStatusCount {
		w.labeledGauge("pipeline_ingest_job_status_count", "Ingest jobs by current status", map[string]string{"status": item.Status}, float64(item.Count))
	}
	for _, item := range health.StageStatusCount {
		w.labeledGauge("pipeline_stage_event_count", "Pipeline events by stage and status in the health window", map[string]string{"stage": item.Stage, "status": item.Status}, float64(item.Count))
	}
	w.labeledGauge("pipeline_security_scan_recent_count", "Security scan events in the health window", map[string]string{"result": "passed"}, float64(health.Security.Passed))
	w.labeledGauge("pipeline_security_scan_recent_count", "Security scan events in the health window", map[string]string{"result": "blocked"}, float64(health.Security.Blocked))
	w.labeledGauge("pipeline_security_scan_recent_count", "Security scan events in the health window", map[string]string{"result": "unavailable"}, float64(health.Security.Unavailable))

	w.gauge("pipeline_publish_latency_average_seconds", "Average upload-to-publish latency for completed uploads in the health window", health.Latency.AverageSeconds)
	w.gauge("pipeline_publish_latency_p50_seconds", "P50 upload-to-publish latency for completed uploads in the health window", health.Latency.P50Seconds)
	w.gauge("pipeline_publish_latency_p95_seconds", "P95 upload-to-publish latency for completed uploads in the health window", health.Latency.P95Seconds)
	w.gauge("pipeline_publish_latency_max_seconds", "Max upload-to-publish latency for completed uploads in the health window", health.Latency.MaxSeconds)
	w.gauge("pipeline_publish_latency_completed_count", "Completed uploads included in publish latency calculations", float64(health.Latency.CompletedCount))

	for _, alert := range health.Alerts {
		w.labeledGauge(
			"pipeline_health_alert",
			"Active pipeline health alerts",
			map[string]string{"code": alert.Code, "severity": alert.Severity},
			1,
		)
	}
	return b.String()
}

func RenderPipelineMetricsUnavailablePrometheus() string {
	var b strings.Builder
	newPrometheusGaugeWriter(&b).gauge("pipeline_metrics_up", "Whether pipeline metrics were collected successfully", 0)
	return b.String()
}

type prometheusGaugeWriter struct {
	b       *strings.Builder
	headers map[string]struct{}
}

func newPrometheusGaugeWriter(b *strings.Builder) *prometheusGaugeWriter {
	return &prometheusGaugeWriter{b: b, headers: map[string]struct{}{}}
}

func (w *prometheusGaugeWriter) gauge(name, help string, value float64) {
	w.header(name, help)
	fmt.Fprintf(w.b, "%s %s\n", name, strconv.FormatFloat(value, 'f', -1, 64))
}

func (w *prometheusGaugeWriter) labeledGauge(name, help string, labels map[string]string, value float64) {
	w.header(name, help)
	fmt.Fprintf(w.b, "%s%s %s\n", name, prometheusLabels(labels), strconv.FormatFloat(value, 'f', -1, 64))
}

func (w *prometheusGaugeWriter) header(name, help string) {
	if _, ok := w.headers[name]; ok {
		return
	}
	w.headers[name] = struct{}{}
	fmt.Fprintf(w.b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w.b, "# TYPE %s gauge\n", name)
}

func prometheusLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sortStrings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, prometheusEscapeLabelValue(labels[key])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func prometheusEscapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func boolGauge(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
