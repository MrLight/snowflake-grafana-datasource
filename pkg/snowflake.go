package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"net/url"

	"github.com/allegro/bigcache/v3"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type DBDataResponse struct {
	dataResponse backend.DataResponse
	refID        string
}

// newDatasource returns datasource.ServeOpts.
func newDatasource() datasource.ServeOpts {
	// creates a instance manager for your plugin. The function passed
	// into `NewInstanceManger` is called when the instance is created
	// for the first time or when a datasource configuration changed.
	im := datasource.NewInstanceManager(newDataSourceInstance)
	ds := &SnowflakeDatasource{
		im: im,
	}

	return datasource.ServeOpts{
		QueryDataHandler:   ds,
		CheckHealthHandler: ds,
	}
}

type SnowflakeDatasource struct {
	// The instance manager can help with lifecycle management
	// of datasource instances in plugins. It's not a requirements
	// but a best practice that we recommend that you follow.
	im            instancemgmt.InstanceManager
	actQueryCount queryCounter
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifer).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (td *SnowflakeDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {

	// create response struct
	result := backend.NewQueryDataResponse()

	/*password := req.PluginContext.DataSourceInstanceSettings.DecryptedSecureJSONData["password"]
	privateKey := req.PluginContext.DataSourceInstanceSettings.DecryptedSecureJSONData["privateKey"]

	config, err := getConfig(req.PluginContext.DataSourceInstanceSettings)
	if err != nil {
		log.DefaultLogger.Error("Could not get config for plugin", "err", err)
		return response, err
	}*/
	i, err := td.im.Get(ctx, req.PluginContext)
	if err != nil {
		return nil, err
	}
	instance := i.(*instanceSettings)
	ch := make(chan DBDataResponse, len(req.Queries))
	var wg sync.WaitGroup
	// Execute each query in a goroutine and wait for them to finish afterwards
	for _, query := range req.Queries {
		wg.Add(1)
		go td.query(ctx, &wg, ch, instance, query)
		//go e.executeQuery(query, &wg, ctx, ch, queryjson)
	}

	wg.Wait()

	// Read results from channels
	close(ch)
	result.Responses = make(map[string]backend.DataResponse)
	for queryResult := range ch {
		result.Responses[queryResult.refID] = queryResult.dataResponse
	}

	return result, nil
}

type pluginConfig struct {
	Account               string `json:"account"`
	Username              string `json:"username"`
	Role                  string `json:"role"`
	Warehouse             string `json:"warehouse"`
	Database              string `json:"database"`
	Schema                string `json:"schema"`
	ExtraConfig           string `json:"extraConfig"`
	MaxOpenConnections    string `json:"maxOpenConnections"`
	IntMaxOpenConnections int64
	ConnectionLifetime    string `json:"connectionLifetime"`
	IntConnectionLifetime int64
	UseCaching            bool   `json:"useCaching"`
	UseCacheByDefault     bool   `json:"useCacheByDefault"`
	CacheSize             string `json:"cacheSize"`
	IntCacheSize          int64
	CacheRetention        string `json:"cacheRetention"`
	IntCacheRetention     int64
}

func getConfig(settings *backend.DataSourceInstanceSettings) (pluginConfig, error) {
	var config pluginConfig
	err := json.Unmarshal(settings.JSONData, &config)
	if config.MaxOpenConnections == "" {
		config.MaxOpenConnections = "100"
	}
	if config.ConnectionLifetime == "" {
		config.ConnectionLifetime = "60"
	}
	if config.CacheSize == "" {
		config.CacheSize = "2048"
	}
	if config.CacheRetention == "" {
		config.CacheRetention = "60"
	}
	if MaxOpenConnections, err := strconv.Atoi(config.MaxOpenConnections); err == nil {
		config.IntMaxOpenConnections = int64(MaxOpenConnections)
	} else {
		return config, err
	}
	if ConnectionLifetime, err := strconv.Atoi(config.ConnectionLifetime); err == nil {
		config.IntConnectionLifetime = int64(ConnectionLifetime)
	} else {
		return config, err
	}
	if CacheSize, err := strconv.Atoi(config.CacheSize); err == nil {
		config.IntCacheSize = int64(CacheSize)
	} else {
		return config, err
	}
	if CacheRetention, err := strconv.Atoi(config.CacheRetention); err == nil {
		config.IntCacheRetention = int64(CacheRetention)
	} else {
		return config, err
	}
	if err != nil {
		return config, err
	}
	return config, nil
}

func getConnectionString(config *pluginConfig, password string, privateKey string) string {
	params := url.Values{}
	params.Add("role", config.Role)
	params.Add("warehouse", config.Warehouse)
	params.Add("database", config.Database)
	params.Add("schema", config.Schema)

	var userPass = ""
	if len(privateKey) != 0 {
		params.Add("authenticator", "SNOWFLAKE_JWT")
		params.Add("privateKey", privateKey)
		userPass = url.User(config.Username).String()
	} else {
		userPass = url.UserPassword(config.Username, password).String()
	}

	return fmt.Sprintf("%s@%s?%s&%s", userPass, config.Account, params.Encode(), config.ExtraConfig)
}

type instanceSettings struct {
	db    *sql.DB
	cache *bigcache.BigCache
}

func newDataSourceInstance(ctx context.Context, setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {

	log.DefaultLogger.Info("Creating instance")
	password := setting.DecryptedSecureJSONData["password"]
	privateKey := setting.DecryptedSecureJSONData["privateKey"]

	config, err := getConfig(&setting)
	if err != nil {
		log.DefaultLogger.Error("Could not get config for plugin", "err", err)
		return nil, err
	}

	connectionString := getConnectionString(&config, password, privateKey)
	db, err := sql.Open("snowflake", connectionString)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(int(config.IntMaxOpenConnections))
	db.SetMaxIdleConns(int(config.IntMaxOpenConnections))
	db.SetConnMaxLifetime(time.Duration(int(config.IntConnectionLifetime)) * time.Minute)

	var cache *bigcache.BigCache = nil
	if config.UseCaching {
		cache_config := bigcache.Config{
			// number of shards (must be a power of 2)
			Shards: 1024,

			// time after which entry can be evicted
			LifeWindow: time.Duration(config.IntCacheRetention) * time.Minute,

			// Interval between removing expired entries (clean up).
			// If set to <= 0 then no action is performed.
			// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
			CleanWindow: 5 * time.Minute,

			// rps * lifeWindow, used only in initial memory allocation
			MaxEntriesInWindow: 1000 * 10 * 60,

			// max entry size in bytes, used only in initial memory allocation
			MaxEntrySize: 500,

			// prints information about additional memory allocation
			Verbose: true,

			// cache will not allocate more memory than this limit, value in MB
			// if value is reached then the oldest entries can be overridden for the new ones
			// 0 value means no size limit
			HardMaxCacheSize: int(config.IntCacheSize),

			// callback fired when the oldest entry is removed because of its expiration time or no space left
			// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
			// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
			OnRemove: nil,

			// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
			// for the new entry, or because delete was called. A constant representing the reason will be passed through.
			// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
			// Ignored if OnRemove is specified.
			OnRemoveWithReason: nil,
		}

		cache, _ = bigcache.New(context.Background(), cache_config)
	}
	return &instanceSettings{db: db, cache: cache}, nil
}

func (s *instanceSettings) Dispose() {
	log.DefaultLogger.Info("Disposing of instance")
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.DefaultLogger.Error("Failed to dispose db", "error", err)
		}
	}
	if s.cache != nil {
		if err := s.cache.Close(); err != nil {
			log.DefaultLogger.Error("Failed to dispose db", "error", err)
		}
	}
	log.DefaultLogger.Debug("DB disposed")
}
