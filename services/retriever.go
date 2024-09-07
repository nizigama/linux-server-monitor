package services

import (
	"github.com/nizigama/linux-server-monitor/structs"
	"gorm.io/gorm"
	"log"
	"time"
)

const (
	everyThirtySeconds structs.Interval = iota
	everyMinute
	everyTenMinute
	everyFifteenMinute
	everyThirtyMinutes
	noLimit
)

func GetMetrics(db *gorm.DB, startDatetime string, endDatetime string) ([]structs.Metrics, error) {

	metrics := []structs.Metrics{
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

	cpuMetrics := []structs.Cpu{}
	memoryMetrics := []structs.Memory{}
	diskMetrics := []structs.Disk{}

	start, err := time.ParseInLocation(time.DateTime, startDatetime, time.Local)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation(time.DateTime, endDatetime, time.Local)
	if err != nil {
		return nil, err
	}

	timeDifference := end.Sub(start)
	interval := noLimit

	if timeDifference.Hours() <= 1 {
		interval = noLimit
	} else if timeDifference.Hours() <= 6 {
		interval = everyThirtySeconds
	} else if timeDifference.Hours() <= 24 {
		interval = everyMinute
	} else if timeDifference.Hours() <= (24 * 7) {
		interval = everyFifteenMinute
	} else if timeDifference.Hours() <= (24 * 14) {
		interval = everyFifteenMinute
	} else if timeDifference.Hours() <= (24 * 21) {
		interval = everyThirtyMinutes
	} else if timeDifference.Hours() <= (24 * 30) {
		interval = everyThirtyMinutes
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

		if len(metrics[0].Data) > 0 {
			lastMetricTimestamp, err := time.Parse(time.DateTime, metrics[0].Data[len(metrics[0].Data)-1][0][0])
			if err != nil {
				log.Println(err)
				return nil, err
			}
			currentMetricTimestamp, err := time.Parse(time.DateTime, metric.Metrics[0][0])
			if err != nil {
				log.Println(err)
				return nil, err
			}

			switch true {
			case interval == everyThirtySeconds:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Seconds() <= 30 {
					continue
				}

				metrics[0].Data = append(metrics[0].Data, metric.Metrics)
			case interval == everyMinute:
				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 1 {
					continue
				}

				metrics[0].Data = append(metrics[0].Data, metric.Metrics)
			case interval == everyTenMinute:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 10 {
					continue
				}

				metrics[0].Data = append(metrics[0].Data, metric.Metrics)
			case interval == everyFifteenMinute:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 15 {
					continue
				}

				metrics[0].Data = append(metrics[0].Data, metric.Metrics)
			case interval == everyThirtyMinutes:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 30 {
					continue
				}

				metrics[0].Data = append(metrics[0].Data, metric.Metrics)
			default:
				metrics[0].Data = append(metrics[0].Data, metric.Metrics)
			}
			continue
		}

		metrics[0].Data = append(metrics[0].Data, metric.Metrics)
	}

	for _, metric := range memoryMetrics {

		if len(metrics[1].Data) > 0 {
			lastMetricTimestamp, err := time.Parse(time.DateTime, metrics[1].Data[len(metrics[1].Data)-1][0][0])
			if err != nil {
				log.Println(err)
				return nil, err
			}
			currentMetricTimestamp, err := time.Parse(time.DateTime, metric.Metrics[0][0])
			if err != nil {
				log.Println(err)
				return nil, err
			}

			switch true {
			case interval == everyThirtySeconds:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Seconds() <= 30 {
					continue
				}

				metrics[1].Data = append(metrics[1].Data, metric.Metrics)
			case interval == everyMinute:
				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 1 {
					continue
				}

				metrics[1].Data = append(metrics[1].Data, metric.Metrics)
			case interval == everyTenMinute:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 10 {
					continue
				}

				metrics[1].Data = append(metrics[1].Data, metric.Metrics)
			case interval == everyFifteenMinute:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 15 {
					continue
				}

				metrics[1].Data = append(metrics[1].Data, metric.Metrics)
			case interval == everyThirtyMinutes:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 30 {
					continue
				}

				metrics[1].Data = append(metrics[1].Data, metric.Metrics)
			default:
				metrics[1].Data = append(metrics[1].Data, metric.Metrics)
			}
			continue
		}

		metrics[1].Data = append(metrics[1].Data, metric.Metrics)
	}

	for _, metric := range diskMetrics {

		if len(metrics[2].Data) > 0 {
			lastMetricTimestamp, err := time.Parse(time.DateTime, metrics[1].Data[len(metrics[2].Data)-1][0][0])
			if err != nil {
				log.Println(err)
				return nil, err
			}
			currentMetricTimestamp, err := time.Parse(time.DateTime, metric.Metrics[0][0])
			if err != nil {
				log.Println(err)
				return nil, err
			}

			switch true {
			case interval == everyThirtySeconds:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Seconds() <= 30 {
					continue
				}

				metrics[2].Data = append(metrics[2].Data, metric.Metrics)
			case interval == everyMinute:
				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 1 {
					continue
				}

				metrics[2].Data = append(metrics[2].Data, metric.Metrics)
			case interval == everyTenMinute:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 10 {
					continue
				}

				metrics[2].Data = append(metrics[2].Data, metric.Metrics)
			case interval == everyFifteenMinute:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 15 {
					continue
				}

				metrics[2].Data = append(metrics[2].Data, metric.Metrics)
			case interval == everyThirtyMinutes:

				if currentMetricTimestamp.Sub(lastMetricTimestamp).Minutes() <= 30 {
					continue
				}

				metrics[2].Data = append(metrics[2].Data, metric.Metrics)
			default:
				metrics[2].Data = append(metrics[2].Data, metric.Metrics)
			}
			continue
		}

		metrics[2].Data = append(metrics[2].Data, metric.Metrics)
	}

	return metrics, nil
}
