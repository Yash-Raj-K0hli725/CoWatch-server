package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type Service struct {
	db *gorm.DB
}

var DB *gorm.DB

var dbService *Service

var (
	database = os.Getenv("DB_NAME")
	password = os.Getenv("DB_PASSWORD")
	username = os.Getenv("DB_USERNAME")
	port     = os.Getenv("DB_PORT")
	host     = os.Getenv("DB_HOST")
)

// New initializes a new db connection (singleton)
func New() *Service {
	if dbService != nil {
		return dbService
	}

	var err error
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		DB, err = openDbWithDSN(dsn)
	} else {
		h := host
		if h == "" {
			h = "localhost"
		}
		p := port
		if p == "" {
			p = "5432"
		}
		portInt, perr := strconv.Atoi(p)
		if perr != nil {
			log.Fatalf("Could not parse DB_PORT environment variable :: %v", perr)
			return nil
		}
		DB, err = openDb(database, username, password, h, portInt)
	}
	if err != nil {
		log.Fatalf("Could not connect to db: %v", err)
	}
	dbService = &Service{db: DB}
	return dbService
}

func openDb(dbName, user, password, host string, dbPort int) (*gorm.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		host, user, password, dbName, dbPort,
	)
	return openDbWithDSN(connStr)
}

func openDbWithDSN(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		TranslateError: false, // Set to true if you want GORM to translate db errors to standard errors
	})
}

// Health checks the health of the db connection
func (s *Service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	// Get underlying *sql.db
	sqlDB, err := s.db.DB()
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("failed to get sql.db: %v", err)
		return stats
	}

	// Ping the db
	if err := sqlDB.PingContext(ctx); err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Printf("db down: %v", err)
		return stats
	}

	// Database is up
	stats["status"] = "up"
	stats["message"] = "It's healthy"

	// Get connection pool stats
	dbStats := sqlDB.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	// Evaluate stats
	if dbStats.OpenConnections > 40 {
		stats["message"] = "The db is experiencing heavy load."
	}
	if dbStats.WaitCount > 1000 {
		stats["message"] = "High number of wait events, possible bottlenecks."
	}
	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many idle connections closed, revise pool settings."
	}
	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Connections closed due to max lifetime, consider revising settings."
	}

	return stats
}

// Close closes the db connection
func (s *Service) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	log.Printf("Disconnected from db")
	return sqlDB.Close()
}

func (s *Service) GetDatabase() *gorm.DB {
	return s.db
}
