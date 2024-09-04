package main

import (
	"encoding/json"
	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/julienschmidt/httprouter"
	"github.com/nizigama/linux-server-monitor/services"
	"github.com/nizigama/linux-server-monitor/types"
	"gorm.io/gorm"
	"log"
	"net/http"
)

func main() {

	db, err := gorm.Open(sqlite.Open("/metrics/database.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Migrate the schema
	err = db.AutoMigrate(&types.Cpu{}, &types.Memory{}, &types.Disk{})
	if err != nil {
		log.Fatal(err)
	}

	go func(db *gorm.DB) {
		services.RecordMetrics(db)
	}(db)

	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		data, _ := json.Marshal(map[string]string{
			"name":    "Linux monitoring agent",
			"version": "1.0.0",
		})
		_, _ = w.Write(data)
	})
	router.GET("/metrics/:start/:end", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
			return
		}

		data, _ := json.Marshal(metrics)
		_, _ = w.Write(data)
	})

	err = http.ListenAndServe(":8090", router)
	if err != nil {
		log.Fatal(err)
	}
}
