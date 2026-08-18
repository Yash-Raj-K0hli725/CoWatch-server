package server

import (
	"StreamRoom/errz"
	"StreamRoom/internal/queue"
	"StreamRoom/internal/rdb"
	"StreamRoom/storage"
	"StreamRoom/util"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	goredis "github.com/redis/go-redis/v9"
)

type Server struct {
	e      *echo.Echo
	rdb    *goredis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Init() error {
	e := echo.New()
	s.e = e
	s.ctx, s.cancel = context.WithCancel(context.Background())

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	//e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
	//	AllowOrigins:     []string{"https://*", "http://*"},
	//	AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
	//	AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	//	AllowCredentials: true,
	//	MaxAge:           300,
	//}))

	/*----echo-config----*/
	r2Client := storage.InitStorage()
	storageService := storage.NewR2MediaService(r2Client, os.Getenv("BUCKET_NAME"))

	s.rdb = rdb.NewClient()
	if err := s.rdb.Ping(s.ctx).Err(); err != nil {
		return fmt.Errorf("unable to reach redis at %s: %w", os.Getenv("REDIS_ADDR"), err)
	}

	streamName := util.EnvOr("QUEUE_STREAM_NAME", "streamroom:transcode-jobs")
	groupName := util.EnvOr("QUEUE_CONSUMER_GROUP", "transcode-workers")
	// Distinct per-process consumer name so multiple API replicas can share
	// the same stream/group without stepping on each other's deliveries.
	transcodeQueue := queue.NewRedisStream(s.rdb, streamName, groupName, "api-"+util.HostnamePID("api"))
	if err := transcodeQueue.EnsureGroup(s.ctx); err != nil {
		return fmt.Errorf("init transcode queue: %w", err)
	}

	e.Server.Handler = s.RegisterRoutes(storageService, transcodeQueue, s.rdb)
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		response := errz.FormatError(err)
		if !c.Response().Committed {
			c.JSON(response.Code, response)
		}
	}

	return nil
}

func (s *Server) Run() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090" //default
	}

	log.Printf("Starting server on port %s", port)
	err := s.e.Start(fmt.Sprintf(":%s", port))
	if !errors.Is(err, http.ErrServerClosed) && err != nil {
		log.Fatalf("Shutting down the server due to error: %v", err)
	}
	log.Println("Server connection pool closed.")
}

// Shutdown stops accepting new work: it cancels the server-lifetime
// context (tearing down the video-events subscriber), shuts the HTTP
// server down within ctx's deadline, and closes the Redis connection.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}

	var err error
	if s.e != nil {
		err = s.e.Shutdown(ctx)
	}
	if s.rdb != nil {
		_ = s.rdb.Close()
	}
	return err
}

func (s *Server) healthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, "good")
}
