package server

import (
	"StreamRoom/db"
	"StreamRoom/errz"
	"StreamRoom/storage"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	e  *echo.Echo
	db *db.Service
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Init() error {
	e := echo.New()
	s.e = e
	s.db = db.New()
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
	e.Server.Handler = s.RegisterRoutes(storageService)
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		response := errz.FormatError(err)
		if !c.Response().Committed {
			c.JSON(response.Code, response)
		}
	}

	//todo add storage cdn (Cloud flare)
	err := os.MkdirAll("./storage", os.ModePerm)
	if err != nil {
		log.Fatalf("failed to initialize storage path on local machine... :: %v", err)
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

func (s *Server) GetServer() *http.Server {
	if s.e != nil {
		return s.e.Server
	}
	return nil
}

//func (s *Server) dbHealth(c echo.Context) error {
//	return c.JSON(http.StatusOK, s.db.Health())
//}
