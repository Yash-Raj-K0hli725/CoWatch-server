package server

import (
	"StreamRoom/internal/handler"
	"StreamRoom/internal/queue"
	"StreamRoom/internal/service"
	storage "StreamRoom/storage"
	"net/http"
	"os"

	echojwt "github.com/labstack/echo-jwt/v4"
	goredis "github.com/redis/go-redis/v9"
)

func (s *Server) RegisterRoutes(storageService *storage.R2MediaService, transcodeQueue queue.Producer, rdb *goredis.Client) http.Handler {

	/*--------prefix---------*/
	apiGroup := s.e.Group("/api")
	apiV1Group := s.e.Group("/api/v1")

	apiV1Group.Use(echojwt.WithConfig(echojwt.Config{
		SigningKey: []byte(os.Getenv("JWT_SECRET_KEY")),
	}))
	//apiV1Group.Use(handlers.AuthMiddleware)

	/*-------------public group---------------------*/
	publicGroup := s.e.Group("/public")

	/*-------------Service Layer------------*/
	videoService := service.NewVideoService(storageService, transcodeQueue, os.Getenv("BUCKET_NAME"))
	roomService := service.NewRoomService(videoService)

	// Bridge worker-published video lifecycle events (PROCESSING/READY/
	// FAILED, over Redis pub/sub) back into this process's in-memory room
	// state + connected websockets.
	roomService.StartVideoEventListener(s.ctx, rdb)

	/*-------------Handler Layer-------------*/
	//##-with auth-##

	//##-without auth-##
	handler.NewRoomsHandler(apiGroup, roomService)
	handler.NewVideoHandler(apiGroup, videoService, roomService, os.Getenv("WEBHOOK_SIGNING_SECRET"))
	publicGroup.GET("/health", s.healthHandler)

	return s.e
}
