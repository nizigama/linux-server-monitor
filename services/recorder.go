package services

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/nizigama/linux-server-monitor/structs"
	"gorm.io/gorm"
)

func RecordMetrics(db *gorm.DB) {

	logger := log.New(os.Stdout, "RECORD METRICS: ", log.LstdFlags)

	ticker := time.NewTicker(15 * time.Second)

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			datetime := time.Now().Unix()

			wg := sync.WaitGroup{}
			wg.Add(3)

			go func() {
				metrics, err := LoadCpuMetrics()
				if err != nil {
					logger.Printf("Failed to load CPU metrics: %v", err)
					wg.Done()
					return
				}

				// Validate metrics before storing
				if len(metrics) == 0 || len(metrics[0]) == 0 {
					logger.Println("Skipping empty CPU metrics")
					wg.Done()
					return
				}

				err = db.Create(&structs.Cpu{
					Datetime: datetime,
					Metrics:  metrics,
				}).Error

				if err != nil {
					logger.Printf("Failed to save CPU metrics: %v", err)
				}

				wg.Done()
			}()

			go func() {
				metrics, err := LoadMemoryMetrics()
				if err != nil {
					logger.Printf("Failed to load Memory metrics: %v", err)
					wg.Done()
					return
				}

				// Validate metrics before storing
				if len(metrics) == 0 || len(metrics[0]) == 0 {
					logger.Println("Skipping empty Memory metrics")
					wg.Done()
					return
				}

				err = db.Create(&structs.Memory{
					Datetime: datetime,
					Metrics:  metrics,
				}).Error

				if err != nil {
					logger.Printf("Failed to save Memory metrics: %v", err)
				}

				wg.Done()
			}()

			go func() {
				metrics, err := LoadDiskMetrics()
				if err != nil {
					logger.Printf("Failed to load Disk metrics: %v", err)
					wg.Done()
					return
				}

				// Validate metrics before storing
				if len(metrics) == 0 || len(metrics[0]) == 0 {
					logger.Println("Skipping empty Disk metrics")
					wg.Done()
					return
				}

				err = db.Create(&structs.Disk{
					Datetime: datetime,
					Metrics:  metrics,
				}).Error

				if err != nil {
					logger.Printf("Failed to save Disk metrics: %v", err)
				}

				wg.Done()
			}()

			wg.Wait()
		}
	}
}
