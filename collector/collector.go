package collector

import (
	"github.com/aporeto-inc/trireme-statistics/influxdb"
	"github.com/aporeto-inc/trireme/collector"
	"go.uber.org/zap"
)

// NewDefaultCollector returns an empty collectorInstance
func NewDefaultCollector() collector.EventCollector {
	zap.L().Info("Using default empty collector")
	return &collector.DefaultCollector{}
}

// NewInfluxDBCollector returns a collector implementation for InfluxDB
func NewInfluxDBCollector(user, pass, url string) collector.EventCollector {
	zap.L().Info("Using Influx collector", zap.String("endpoint", url), zap.String("user", user))
	collectorInstance, err := influxdb.NewDBConnection(user, pass, url)
	if err != nil {
		zap.L().Error("Error instantiating Influx collector, using default", zap.String("endpoint", url), zap.String("user", user), zap.Error(err))
		return NewDefaultCollector()
	}
	collectorInstance.Start()
	return collectorInstance
}
