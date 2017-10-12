package collector

import (
	"github.com/aporeto-inc/trireme-statistics/collector/influxdb"
	"github.com/aporeto-inc/trireme/collector"
	"go.uber.org/zap"
)

// NewDefaultCollector returns an empty collectorInstance
func NewDefaultCollector() collector.EventCollector {
	zap.L().Info("Using default empty collector")
	return nil
}

// NewInfluxDBCollector returns a collector implementation for InfluxDB
func NewInfluxDBCollector(user, pass, url string) collector.EventCollector {
	zap.L().Info("Using Influx collector", zap.String("endpoint", url), zap.String("user", user))
	collectorInstance := influxdb.CreateAndConnectDB(user, pass, url)
	return collectorInstance
}
