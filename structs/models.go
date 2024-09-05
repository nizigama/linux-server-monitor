package structs

import "gorm.io/gorm"

type Cpu struct {
	gorm.Model
	Datetime int64
	Metrics  [][]string `gorm:"serializer:json"`
}

type Memory struct {
	gorm.Model
	Datetime int64
	Metrics  [][]string `gorm:"serializer:json"`
}

type Disk struct {
	gorm.Model
	Datetime int64
	Metrics  [][]string `gorm:"serializer:json"`
}
