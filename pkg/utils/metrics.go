package utils

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CacheMetrics is implemented by queryCache in the main package.
// Using an interface avoids a circular import.
type CacheMetrics interface {
	Len() int64
	Hits() uint64
	Misses() uint64
	CostUsed() int64
}

type LocalPrometheusCollector struct {
	db                        *sql.DB
	cache                     CacheMetrics
	db_in_use_connections     *prometheus.Desc
	db_idle_connections       *prometheus.Desc
	db_open_connections       *prometheus.Desc
	db_idle_connections_total *prometheus.Desc
	db_idle_time_total        *prometheus.Desc
	db_max_open_connections   *prometheus.Desc
	db_wait_count             *prometheus.Desc
	db_wait_duration          *prometheus.Desc
	cache_act_cost_used       *prometheus.Desc
	cache_act_size            *prometheus.Desc
	cache_hits_total          *prometheus.Desc
	cache_miss_total          *prometheus.Desc
}

func (c *LocalPrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.db_in_use_connections
	ch <- c.db_idle_connections
	ch <- c.db_open_connections
	ch <- c.db_idle_connections_total
	ch <- c.db_idle_time_total
	ch <- c.db_max_open_connections
	ch <- c.db_wait_count
	ch <- c.db_wait_duration
	ch <- c.cache_act_cost_used
	ch <- c.cache_act_size
	ch <- c.cache_hits_total
	ch <- c.cache_miss_total
}

func (c *LocalPrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.db_in_use_connections, prometheus.GaugeValue, float64(c.db.Stats().InUse))
	ch <- prometheus.MustNewConstMetric(c.db_idle_connections, prometheus.GaugeValue, float64(c.db.Stats().Idle))
	ch <- prometheus.MustNewConstMetric(c.db_open_connections, prometheus.GaugeValue, float64(c.db.Stats().OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.db_idle_connections_total, prometheus.CounterValue, float64(c.db.Stats().MaxIdleClosed))
	ch <- prometheus.MustNewConstMetric(c.db_idle_time_total, prometheus.CounterValue, float64(c.db.Stats().MaxIdleTimeClosed))
	ch <- prometheus.MustNewConstMetric(c.db_max_open_connections, prometheus.GaugeValue, float64(c.db.Stats().MaxOpenConnections))
	ch <- prometheus.MustNewConstMetric(c.db_wait_count, prometheus.CounterValue, float64(c.db.Stats().WaitCount))
	ch <- prometheus.MustNewConstMetric(c.db_wait_duration, prometheus.CounterValue, float64(c.db.Stats().WaitDuration))
	if c.cache != nil {
		ch <- prometheus.MustNewConstMetric(c.cache_act_cost_used, prometheus.GaugeValue, float64(c.cache.CostUsed()))
		ch <- prometheus.MustNewConstMetric(c.cache_act_size, prometheus.GaugeValue, float64(c.cache.Len()))
		ch <- prometheus.MustNewConstMetric(c.cache_hits_total, prometheus.CounterValue, float64(c.cache.Hits()))
		ch <- prometheus.MustNewConstMetric(c.cache_miss_total, prometheus.CounterValue, float64(c.cache.Misses()))
	}
}

func NewLocalPrometheusCollector(db *sql.DB, cache CacheMetrics, setting *backend.DataSourceInstanceSettings) *LocalPrometheusCollector {
	prom_name := ToSnakeCase(setting.Name)
	prom_name = strings.ReplaceAll(prom_name, "-", "_")
	return &LocalPrometheusCollector{
		db:                        db,
		cache:                     cache,
		db_in_use_connections:     prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_in_use_connections", "SQL Pool - The number of connections currently in use.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_idle_connections:       prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_idle_connections", "SQL Pool - The number of idle connections.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_open_connections:       prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_open_connections", "SQL Pool - The number of currently open connections. Pool Status", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_idle_connections_total: prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_idle_connections_total", "SQL Pool - The total number of connections closed due to SetMaxIdleConns.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_idle_time_total:        prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_idle_timeout_connections_total", "SQL Pool - The total number of connections closed due to SetConnMaxIdleTime.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_max_open_connections:   prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_max_connections", "SQL Pool - Maximum number of open connections to the database.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_wait_count:             prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_count_total", "SQL Pool - Connections total wait count", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		db_wait_duration:          prometheus.NewDesc("grafana_plugin_"+prom_name+"_sql_pool_duration_total", "SQL Pool - The total time blocked waiting for a new connection.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_act_cost_used:       prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_act_cost_used", "Cache - Total cost (bytes) currently stored in the cache.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_act_size:            prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_act_size", "Cache - Number of entries currently in the cache.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_hits_total:          prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_hits_total", "Cache - Total number of cache hits.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_miss_total:          prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_miss_total", "Cache - Total number of cache misses.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name})}
}

var QueriesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "grafana_plugin",
		Name:      "queries_total",
		Help:      "Total number of queries.",
		//ConstLabels: prometheus.Labels{"UID": setting.UID, "Name": setting.Name},
	},
	[]string{"query_type", "query_source"},
)

var matchFirstCap = regexp.MustCompile("(.)((<!_)[A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ReplaceAll(strings.ToLower(snake), "-", "_")
}
