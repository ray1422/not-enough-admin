package db

import (
	"log"
	"os"
	"time"

	"github.com/ray1422/not-enough-admin/util"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db  *gorm.DB
	dsn = "host=localhost user=postgres dbname=" + util.Getenv("DB_NAME", "myadmin") + " port=5432 sslmode=disable TimeZone=Asia/Taipei" // TODO read from env var
)

// GormDB return gorm.DB instance. if UNITTEST_DB_TX is not empty, a session is returned and it's for unit test rollback.
func GormDB() *gorm.DB {
	newLogger := logger.New(
		log.New(os.Stdout, "\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             1000 * time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info,        // Log level
			IgnoreRecordNotFoundError: true,               // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,              // Disable color
		},
	)

	if db == nil {
		db2, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: newLogger,
		})
		if err != nil {
			panic(err)
		}
		db = db2
		if os.Getenv("UNITTEST_DB_TX") != "" {
			db = db.Begin()
		}
	}

	return db
}
