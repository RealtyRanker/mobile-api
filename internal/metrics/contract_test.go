package metrics

import (
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Freeze the existing public metric names, types, labels and bucket boundaries.
// Grafana queries and Prometheus jobs depend on this contract.
func TestMonitoringContract(t *testing.T) {
	RequestsTotal.WithLabelValues("test", "test").Add(0)
	RequestDuration.WithLabelValues("test").Observe(0)
	SignInsTotal.WithLabelValues("test").Add(0)
	expected := map[string]struct {
		kind    string
		labels  []string
		buckets []float64
	}{
		"mobileapi_requests_total":           {kind: "COUNTER", labels: []string{"route", "status"}, buckets: []float64{}},
		"mobileapi_request_duration_seconds": {kind: "HISTOGRAM", labels: []string{"route"}, buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}},
		"mobileapi_sign_ins_total":           {kind: "COUNTER", labels: []string{"outcome"}, buckets: []float64{}},
	}
	gathered, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range gathered {
		contract, ok := expected[family.GetName()]
		if !ok {
			continue
		}
		if family.GetType().String() != contract.kind {
			t.Errorf("%s type changed", family.GetName())
		}
		for _, metric := range family.Metric {
			labels := []string{}
			for _, label := range metric.Label {
				labels = append(labels, label.GetName())
			}
			sort.Strings(labels)
			if !reflect.DeepEqual(labels, contract.labels) {
				t.Errorf("%s labels changed: %v", family.GetName(), labels)
			}
			if contract.kind == "HISTOGRAM" {
				var bounds []float64
				for _, bucket := range metric.GetHistogram().Bucket {
					bounds = append(bounds, bucket.GetUpperBound())
				}
				if !reflect.DeepEqual(bounds, contract.buckets) {
					t.Errorf("%s buckets changed: %v", family.GetName(), bounds)
				}
			}
		}
		delete(expected, family.GetName())
	}
	if len(expected) > 0 {
		t.Fatalf("missing metrics: %v", expected)
	}
	response := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "# TYPE") {
		t.Fatalf("metrics endpoint: %d", response.Code)
	}
}
