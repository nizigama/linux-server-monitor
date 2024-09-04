package services

import (
	"github.com/nizigama/linux-server-monitor/types"
	"gorm.io/gorm"
	"time"
)

type Metrics struct {
	Type string
	Data [][][]string
}

func GetMetrics(db *gorm.DB, startDatetime string, endDatetime string) ([]Metrics, error) {

	metrics := []Metrics{
		{
			Type: "Cpu",
			Data: [][][]string{},
		},
		{
			Type: "Memory",
			Data: [][][]string{},
		},
		{
			Type: "Disk",
			Data: [][][]string{},
		},
	}

	cpuMetrics := []types.Cpu{}
	memoryMetrics := []types.Memory{}
	diskMetrics := []types.Disk{}

	start, err := time.ParseInLocation(time.DateTime, startDatetime, time.Local)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation(time.DateTime, endDatetime, time.Local)
	if err != nil {
		return nil, err
	}

	err = db.Where("datetime >= ? AND datetime < ?", start.Unix(), end.Unix()).Select("metrics").Find(&cpuMetrics).Error
	if err != nil {
		return nil, err
	}

	err = db.Where("datetime >= ? AND datetime < ?", start.Unix(), end.Unix()).Select("metrics").Find(&memoryMetrics).Error
	if err != nil {
		return nil, err
	}

	err = db.Where("datetime >= ? AND datetime < ?", start.Unix(), end.Unix()).Select("metrics").Find(&diskMetrics).Error
	if err != nil {
		return nil, err
	}

	for _, metric := range cpuMetrics {
		metrics[0].Data = append(metrics[0].Data, metric.Metrics)
	}

	for _, metric := range memoryMetrics {
		metrics[1].Data = append(metrics[1].Data, metric.Metrics)
	}

	for _, metric := range diskMetrics {
		metrics[2].Data = append(metrics[2].Data, metric.Metrics)
	}

	return metrics, nil
}
