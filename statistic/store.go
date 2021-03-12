package statistic

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	dbPath = "statistic.sqlite3"
)

var db *gorm.DB

func InitStatistic(debug bool) {
	var err error
	conf := &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Discard,
	}
	if debug {
		conf.Logger = logger.Default
	}
	db, err = gorm.Open(sqlite.Open(dbPath), conf)
	if err != nil {
		panic(err)
	}
	err = db.AutoMigrate(
		&CollectRecord{},
		&StatRecord{},
		&StatCounter{},
		&IPCounter{},
		&UserAgentCounter{},
		&MethodCounter{},
		&MethodParamsCounter{},
	)
	if err != nil {
		panic(err)
	}
}
