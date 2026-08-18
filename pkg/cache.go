package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	_data "github.com/michelin/snowflake-grafana-datasource/pkg/data"
)

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// queryCache wraps ristretto and stores frame pointers directly.
type queryCache struct {
	client    *ristretto.Cache[string, *data.Frame]
	retention time.Duration
}

func (c *queryCache) Len() int64 {
	if c == nil || c.client == nil {
		return 0
	}
	return int64(c.client.Metrics.KeysAdded()) - int64(c.client.Metrics.KeysEvicted())
}

func (c *queryCache) Hits() uint64 {
	if c == nil || c.client == nil {
		return 0
	}
	return c.client.Metrics.Hits()
}

func (c *queryCache) Misses() uint64 {
	if c == nil || c.client == nil {
		return 0
	}
	return c.client.Metrics.Misses()
}

func (c *queryCache) CostUsed() int64 {
	if c == nil || c.client == nil {
		return 0
	}
	return int64(c.client.Metrics.CostAdded()) - int64(c.client.Metrics.CostEvicted())
}

func (c *queryCache) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func newQueryCache(config pluginConfig) (*queryCache, error) {
	if !config.UseCaching {
		return nil, nil
	}

	cacheSizeMB := 2048
	cacheRetentionMin := 60

	if config.CacheSize == "" {
		config.CacheSize = "2048"
	}
	if config.CacheRetention == "" {
		config.CacheRetention = "60"
	}

	if v, err := strconv.Atoi(config.CacheSize); err == nil {
		cacheSizeMB = v
	} else {
		return nil, err
	}
	if v, err := strconv.Atoi(config.CacheRetention); err == nil {
		cacheRetentionMin = v
	} else {
		return nil, err
	}

	// MaxCost is the hard memory limit in bytes.
	maxCost := int64(cacheSizeMB) * 1024 * 1024

	rc, err := ristretto.NewCache(&ristretto.Config[string, *data.Frame]{
		NumCounters: 1e6,     // ~10× expected number of cached items
		MaxCost:     maxCost, // hard memory limit in bytes
		BufferItems: 64,      // recommended default
		Metrics:     true,
	})
	if err != nil {
		return nil, err
	}

	return &queryCache{
		client:    rc,
		retention: time.Duration(cacheRetentionMin) * time.Minute,
	}, nil
}

func getQueryFromCache(cache *queryCache, queryConfig _data.QueryConfigStruct) (*data.Frame, error) {
	if cache == nil || !queryConfig.CacheState.Use {
		return data.NewFrame(""), errors.New("noCache")
	}
	key := GetMD5Hash(queryConfig.CacheState.Until.Format(time.RFC3339) + queryConfig.FinalQuery)
	frame, ok := cache.client.Get(key)
	if !ok {
		return data.NewFrame(""), errors.New("Entry not found")
	}
	log.DefaultLogger.Info("Snowflake cache hit")
	return frame, nil
}

func setQueryInCache(cache *queryCache, queryConfig _data.QueryConfigStruct, frame *data.Frame) error {
	if cache == nil || !queryConfig.CacheState.Use {
		return errors.New("noCache")
	}
	key := GetMD5Hash(queryConfig.CacheState.Until.Format(time.RFC3339) + queryConfig.FinalQuery)
	cache.client.SetWithTTL(key, frame, estimateFrameCost(frame), cache.retention)
	cache.client.Wait()
	return nil
}

func estimateFrameCost(frame *data.Frame) int64 {
	if frame == nil {
		return 1
	}

	var cost int64
	for _, field := range frame.Fields {
		if field == nil {
			continue
		}

		switch field.Type() {
		case data.FieldTypeNullableString, data.FieldTypeString:
			for i := 0; i < field.Len(); i++ {
				if value, ok := field.ConcreteAt(i); ok {
					if text, ok := value.(string); ok {
						cost += int64(len(text))
						continue
					}
				}
				cost += 16
			}
		case data.FieldTypeNullableTime, data.FieldTypeNullableFloat64, data.FieldTypeNullableInt64, data.FieldTypeNullableUint64, data.FieldTypeNullableBool:
			cost += int64(field.Len()) * 8
		default:
			cost += int64(field.Len()) * 16
		}
	}

	if cost <= 0 {
		return 1
	}
	return cost
}
