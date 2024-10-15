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
	setting                   *backend.DataSourceInstanceSettings
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
	ch <- prometheus.MustNewConstMetric(c.db_in_use_connections, prometheus.GaugeValue, float64(c.db.Stats().InUse), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_idle_connections, prometheus.GaugeValue, float64(c.db.Stats().Idle), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_open_connections, prometheus.GaugeValue, float64(c.db.Stats().OpenConnections), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_idle_connections_total, prometheus.CounterValue, float64(c.db.Stats().MaxIdleClosed), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_idle_time_total, prometheus.CounterValue, float64(c.db.Stats().MaxIdleTimeClosed), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_max_open_connections, prometheus.GaugeValue, float64(c.db.Stats().MaxOpenConnections), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_wait_count, prometheus.CounterValue, float64(c.db.Stats().WaitCount), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.db_wait_duration, prometheus.CounterValue, float64(c.db.Stats().WaitDuration), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_act_capacity, prometheus.GaugeValue, float64(c.cache.Capacity()), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_act_size, prometheus.GaugeValue, float64(c.cache.Len()), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_collisions_total, prometheus.CounterValue, float64(c.cache.Stats().Collisions), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_del_hits_total, prometheus.CounterValue, float64(c.cache.Stats().DelHits), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_del_miss_total, prometheus.CounterValue, float64(c.cache.Stats().DelMisses), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_hits_total, prometheus.CounterValue, float64(c.cache.Stats().Hits), c.setting.Name)
	ch <- prometheus.MustNewConstMetric(c.cache_miss_total, prometheus.CounterValue, float64(c.cache.Stats().Misses), c.setting.Name)
}

func NewLocalPrometheusCollector(db *sql.DB, cache *bigcache.BigCache, setting *backend.DataSourceInstanceSettings) *LocalPrometheusCollector {
	//prom_name := ToSnakeCase(setting.Name)
	//prom_name = strings.ReplaceAll(prom_name, "-", "_")
	return &LocalPrometheusCollector{
		db:                        db,
		cache:                     cache, //nil
		setting:                   setting,
		db_in_use_connections:     prometheus.NewDesc("grafana_plugin_sql_pool_in_use_connections", "SQL Pool - The number of connections currently in use.", []string{"instance_name"}, nil),
		db_idle_connections:       prometheus.NewDesc("grafana_plugin_sql_pool_idle_connections", "SQL Pool - The number of idle connections.", []string{"instance_name"}, nil),
		db_open_connections:       prometheus.NewDesc("grafana_plugin_sql_pool_open_connections", "SQL Pool - The number of currently open connections. Pool Status", []string{"instance_name"}, nil),
		db_idle_connections_total: prometheus.NewDesc("grafana_plugin_sql_pool_idle_connections_total", "SQL Pool - The total number of connections closed due to SetMaxIdleConns.", []string{"instance_name"}, nil),
		db_idle_time_total:        prometheus.NewDesc("grafana_plugin_sql_pool_idle_timeout_connections_total", "SQL Pool - The total number of connections closed due to SetConnMaxIdleTime.", []string{"instance_name"}, nil),
		db_max_open_connections:   prometheus.NewDesc("grafana_plugin_sql_pool_max_connections", "SQL Pool - Maximum number of open connections to the database.", []string{"instance_name"}, nil),
		db_wait_count:             prometheus.NewDesc("grafana_plugin_sql_pool_count_total", "SQL Pool - Connections total wait count", []string{"instance_name"}, nil),
		db_wait_duration:          prometheus.NewDesc("grafana_plugin_sql_pool_duration_total", "SQL Pool - The total time blocked waiting for a new connection.", []string{"instance_name"}, nil),
		cache_act_capacity:        prometheus.NewDesc("grafana_plugin_cache_act_capacity", "Cache - Capacity returns amount of bytes store in the cache.", []string{"instance_name"}, nil),
		cache_act_size:            prometheus.NewDesc("grafana_plugin_cache_act_size", "Cache Len computes number of entries in cache- ", []string{"instance_name"}, nil),
		cache_collisions_total:    prometheus.NewDesc("grafana_plugin_cache_collisions_total", "Cache - Collisions is a number of happened key-collisions", []string{"instance_name"}, nil),
		cache_del_hits_total:      prometheus.NewDesc("grafana_plugin_cache_del_hits_total", "Cache - DelHits is a number of successfully deleted keys", []string{"instance_name"}, nil),
		cache_del_miss_total:      prometheus.NewDesc("grafana_plugin_cache_del_miss_total", "Cache - DelMisses is a number of not deleted keys", []string{"instance_name"}, nil),
		cache_hits_total:          prometheus.NewDesc("grafana_plugin_cache_hits_total", "Cache - Hits is a number of successfully found keys", []string{"instance_name"}, nil),
		cache_miss_total:          prometheus.NewDesc("grafana_plugin_cache_miss_total", "Cache - Misses is a number of not found keys", []string{"instance_name"}, nil)}
}

func NewLocalPrometheusCollector_() *LocalPrometheusCollector {
	//prom_name := ToSnakeCase(setting.Name)
	//prom_name = strings.ReplaceAll(prom_name, "-", "_")
	return &LocalPrometheusCollector{
		db_in_use_connections:     prometheus.NewDesc("grafana_plugin_sql_pool_in_use_connections", "SQL Pool - The number of connections currently in use.", []string{"instance_name"}, nil),
		db_idle_connections:       prometheus.NewDesc("grafana_plugin_sql_pool_idle_connections", "SQL Pool - The number of idle connections.", []string{"instance_name"}, nil),
		db_open_connections:       prometheus.NewDesc("grafana_plugin_sql_pool_open_connections", "SQL Pool - The number of currently open connections. Pool Status", []string{"instance_name"}, nil),
		db_idle_connections_total: prometheus.NewDesc("grafana_plugin_sql_pool_idle_connections_total", "SQL Pool - The total number of connections closed due to SetMaxIdleConns.", []string{"instance_name"}, nil),
		db_idle_time_total:        prometheus.NewDesc("grafana_plugin_sql_pool_idle_timeout_connections_total", "SQL Pool - The total number of connections closed due to SetConnMaxIdleTime.", []string{"instance_name"}, nil),
		db_max_open_connections:   prometheus.NewDesc("grafana_plugin_sql_pool_max_connections", "SQL Pool - Maximum number of open connections to the database.", []string{"instance_name"}, nil),
		db_wait_count:             prometheus.NewDesc("grafana_plugin_sql_pool_count_total", "SQL Pool - Connections total wait count", []string{"instance_name"}, nil),
		db_wait_duration:          prometheus.NewDesc("grafana_plugin_sql_pool_duration_total", "SQL Pool - The total time blocked waiting for a new connection.", []string{"instance_name"}, nil),
		cache_act_capacity:        prometheus.NewDesc("grafana_plugin_cache_act_capacity", "Cache - Capacity returns amount of bytes store in the cache.", []string{"instance_name"}, nil),
		cache_act_size:            prometheus.NewDesc("grafana_plugin_cache_act_size", "Cache Len computes number of entries in cache- ", []string{"instance_name"}, nil),
		cache_collisions_total:    prometheus.NewDesc("grafana_plugin_cache_collisions_total", "Cache - Collisions is a number of happened key-collisions", []string{"instance_name"}, nil),
		cache_del_hits_total:      prometheus.NewDesc("grafana_plugin_cache_del_hits_total", "Cache - DelHits is a number of successfully deleted keys", []string{"instance_name"}, nil),
		cache_del_miss_total:      prometheus.NewDesc("grafana_plugin_cache_del_miss_total", "Cache - DelMisses is a number of not deleted keys", []string{"instance_name"}, nil),
		cache_hits_total:          prometheus.NewDesc("grafana_plugin_cache_hits_total", "Cache - Hits is a number of successfully found keys", []string{"instance_name"}, nil),
		cache_miss_total:          prometheus.NewDesc("grafana_plugin_cache_miss_total", "Cache - Misses is a number of not found keys", []string{"instance_name"}, nil)}
}

var queriesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "grafana_plugin",
		Name:      "queries_total",
		Help:      "Total number of queries.",
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
