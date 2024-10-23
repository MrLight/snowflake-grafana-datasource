package main

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/allegro/bigcache/v3"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type LocalPrometheusCollector struct {
	db                        *sql.DB
	cache                     *bigcache.BigCache
	db_in_use_connections     *prometheus.Desc
	db_idle_connections       *prometheus.Desc
	db_open_connections       *prometheus.Desc
	db_idle_connections_total *prometheus.Desc
	db_idle_time_total        *prometheus.Desc
	db_max_open_connections   *prometheus.Desc
	db_wait_count             *prometheus.Desc
	db_wait_duration          *prometheus.Desc
	cache_act_capacity        *prometheus.Desc
	cache_act_size            *prometheus.Desc
	cache_collisions_total    *prometheus.Desc
	cache_del_hits_total      *prometheus.Desc
	cache_del_miss_total      *prometheus.Desc
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
	ch <- c.cache_act_capacity
	ch <- c.cache_act_size
	ch <- c.cache_collisions_total
	ch <- c.cache_del_hits_total
	ch <- c.cache_del_miss_total
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
	ch <- prometheus.MustNewConstMetric(c.cache_act_capacity, prometheus.GaugeValue, float64(c.cache.Capacity()))
	ch <- prometheus.MustNewConstMetric(c.cache_act_size, prometheus.GaugeValue, float64(c.cache.Len()))
	ch <- prometheus.MustNewConstMetric(c.cache_collisions_total, prometheus.CounterValue, float64(c.cache.Stats().Collisions))
	ch <- prometheus.MustNewConstMetric(c.cache_del_hits_total, prometheus.CounterValue, float64(c.cache.Stats().DelHits))
	ch <- prometheus.MustNewConstMetric(c.cache_del_miss_total, prometheus.CounterValue, float64(c.cache.Stats().DelMisses))
	ch <- prometheus.MustNewConstMetric(c.cache_hits_total, prometheus.CounterValue, float64(c.cache.Stats().Hits))
	ch <- prometheus.MustNewConstMetric(c.cache_miss_total, prometheus.CounterValue, float64(c.cache.Stats().Misses))
}

func NewLocalPrometheusCollector(db *sql.DB, cache *bigcache.BigCache, setting *backend.DataSourceInstanceSettings) *LocalPrometheusCollector {
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
		cache_act_capacity:        prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_act_capacity", "Cache - Capacity returns amount of bytes store in the cache.", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_act_size:            prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_act_size", "Cache Len computes number of entries in cache- ", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_collisions_total:    prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_collisions_total", "Cache - Collisions is a number of happened key-collisions", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_del_hits_total:      prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_del_hits_total", "Cache - DelHits is a number of successfully deleted keys", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_del_miss_total:      prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_del_miss_total", "Cache - DelMisses is a number of not deleted keys", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_hits_total:          prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_hits_total", "Cache - Hits is a number of successfully found keys", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name}),
		cache_miss_total:          prometheus.NewDesc("grafana_plugin_"+prom_name+"_cache_miss_total", "Cache - Misses is a number of not found keys", nil, prometheus.Labels{"UID": setting.UID, "Name": setting.Name})}
}

var queriesTotal = promauto.NewCounterVec(
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
