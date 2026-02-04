package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/julienschmidt/httprouter"
	"github.com/nizigama/linux-server-monitor/services"
	"github.com/nizigama/linux-server-monitor/structs"
	"gorm.io/gorm"
)

func main() {

	db, err := gorm.Open(sqlite.Open("/metrics/database.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Migrate the schema
	err = db.AutoMigrate(&structs.Cpu{}, &structs.Memory{}, &structs.Disk{})
	if err != nil {
		log.Fatal(err)
	}

	go func(db *gorm.DB) {
		services.RecordMetrics(db)
	}(db)

	if os.Getenv("METRICS_SYNC_DISABLED") != "true" {
		go services.SyncMetrics(db)
	}

	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		data, _ := json.Marshal(map[string]string{
			"name":    "Linux monitoring agent",
			"version": "1.0.0",
		})
		_, _ = w.Write(data)
	})
	// CORS-enabled metrics endpoint
	router.GET("/metrics/:start/:end", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		startDatetime := params.ByName("start")
		endDatetime := params.ByName("end")

		validate := validator.New()

		err := validate.Struct(struct {
			StartDatetime string `validate:"required,datetime=2006-01-02 15:04:05"`
			EndDatetime   string `validate:"required,datetime=2006-01-02 15:04:05"`
		}{
			StartDatetime: startDatetime,
			EndDatetime:   endDatetime,
		})
		if err != nil {
			var validationErrors []string
			for _, err := range err.(validator.ValidationErrors) {
				validationErrors = append(validationErrors, err.Error())
			}
			w.WriteHeader(http.StatusBadRequest)
			out, _ := json.Marshal(validationErrors)
			_, _ = w.Write(out)
			log.Println("Bad request", validationErrors)
			return
		}

		metrics, err := services.GetMetrics(db, startDatetime, endDatetime)
		if err != nil {
			response := map[string]string{
				"message": "Failed to get metrics",
			}
			w.WriteHeader(http.StatusInternalServerError)
			out, _ := json.Marshal(response)
			_, _ = w.Write(out)
			log.Println("Failed to get metrics")
			return
		}

		data, _ := json.Marshal(metrics)
		_, _ = w.Write(data)
		log.Println("Fetched metrics...")
	})

	serverPort := os.Getenv("LINUX_SERVER_MONITOR_PORT")
	port := 8090

	envPort, _ := strconv.Atoi(serverPort)
	if envPort > 1024 && envPort < 65535 {
		port = envPort
	}

	log.Println("Listening on port: ", port)

	err = http.ListenAndServe(fmt.Sprintf(":%v", port), router)
	if err != nil {
		log.Fatal(err)
	}
}
