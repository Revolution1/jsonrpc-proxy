package statistic

import (
	"gorm.io/gorm"
)

type CollectRecord struct {
	gorm.Model
	Result string
}

type StatRecord struct {
	gorm.Model
	IP        string
	UserAgent string
	Method    string
	Params    string
}

type StatCounter struct {
	gorm.Model
	Stat  uint
	Count int64
}

type IPCounter struct {
	gorm.Model
	IP    string
	Count int64
}

type UserAgentCounter struct {
	gorm.Model
	UserAgent string
	Count     int64
}

type MethodCounter struct {
	gorm.Model
	Method string
	Count  int64
}

type MethodParamsCounter struct {
	gorm.Model
	Method string
	Params string
	Count  int64
}
