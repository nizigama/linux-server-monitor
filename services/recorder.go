package services

import (
	"github.com/nizigama/linux-server-monitor/types"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"time"
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
				metrics, _ := LoadCpuMetrics()

				err := db.Create(&types.Cpu{
					Datetime: datetime,
					Metrics:  metrics,
				}).Error

				if err != nil {
					logger.Println(err)
				}

				wg.Done()
			}()

			go func() {
				metrics, _ := LoadMemoryMetrics()

				err := db.Create(&types.Memory{
					Datetime: datetime,
					Metrics:  metrics,
				}).Error

				if err != nil {
					logger.Println(err)
				}

				wg.Done()
			}()

			go func() {
				metrics, _ := LoadDiskMetrics()

				err := db.Create(&types.Disk{
					Datetime: datetime,
					Metrics:  metrics,
				}).Error

				if err != nil {
					logger.Println(err)
				}

				wg.Done()
			}()

			wg.Wait()
		}
	}
}
