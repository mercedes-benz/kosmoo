// SPDX-License-Identifier: MIT

package metrics

import (
	"fmt"
	"strings"
)

// DefaultMetricsPrefix is the default prefix used for the exposed metrics.
const DefaultMetricsPrefix = "kos"

var (
	metricsPrefix string
)

// RegisterMetrics creates and registers the metrics for the /metrics endpoint.
// It needs to get called before any scraping activity.
func RegisterMetrics(prefix string) {
	metricsPrefix = prefix
	registerCinderMetrics()
}

// AddPrefix adds the given prefix to the string, if set
func AddPrefix(name, prefix string) string {
	if prefix == "" {
		return name
	}
	return strings.ToLower(fmt.Sprintf("%s_%s", prefix, name))
}

func generateName(name string) string {
	return AddPrefix(name, metricsPrefix)
}
