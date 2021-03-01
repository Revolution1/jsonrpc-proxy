package statistic

import (
	"gorm.io/gorm"
	"time"
)

type RequestStat struct {
	gorm.Model
	Address   string
	Method    string
	params    string
	Hit       bool
	Useragent string
	Duration  time.Duration
	Count     uint64
}
