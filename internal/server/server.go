package server

import (
	"StreamRoom/db"
	"StreamRoom/errz"
	"StreamRoom/internal/domain/mq"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	e        *echo.Echo
	db       *db.Service
	producer *mq.Producer
	consumer *mq.Consumer
}

const RabbitBaseURL = "amqp://guest:guest@127.0.0.1:5672/"

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
	var err error
	s.producer, err = mq.NewProducer("add", RabbitBaseURL)
	s.consumer = mq.NewConsumer("add", RabbitBaseURL)

	s.RegisterRoutes(s.producer)
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		response := errz.FormatError(err)
		if !c.Response().Committed {
			c.JSON(response.Code, response)
		}
	}

	//todo add storage cdn (Cloud flare)
	err = os.MkdirAll("./storage", os.ModePerm)
	if err != nil {
		log.Fatalf("failed to initialize storage path on local machine... :: %v", err)
	}
	return nil
}

func (s *Server) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	defer stop()

	go func() {
		if err := s.consumer.Start(ctx); err != nil {
			log.Errorf("failed to initiate rabbitM queue : %v", err)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090" //default
	}

	go func() {
		log.Printf("Starting server on port %s", port)
		err := s.e.Start(fmt.Sprintf(":%s", port))
		if !errors.Is(err, http.ErrServerClosed) && err != nil {
			log.Fatalf("Shutting down the server due to error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("Shutdown signal received. Starting graceful drain...")
	s.gracefulShutdown()
}

func (s *Server) GetServer() *http.Server {
	if s.e != nil {
		return s.e.Server
	}
	return nil
}

func (s *Server) gracefulShutdown() {
	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.e.Shutdown(shutdownContext); err != nil {
		log.Printf("Server forced to shutdown with error: %v", err)
	}
	if s.producer != nil {
		s.producer.Close()
	}
	log.Print("Server exiting")
}

//func (s *Server) dbHealth(c echo.Context) error {
//	return c.JSON(http.StatusOK, s.db.Health())
//}
