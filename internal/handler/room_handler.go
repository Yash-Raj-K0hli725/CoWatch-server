package handler

import (
	"StreamRoom/errz"
	"StreamRoom/internal/domain"
	"StreamRoom/internal/service"
	"StreamRoom/internal/views"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type RoomsHandler struct {
	s *service.RoomService
}

func NewRoomsHandler(group *echo.Group, service *service.RoomService) *RoomsHandler {
	h := &RoomsHandler{
		s: service,
	}
	group.POST("/room/create", h.CreateNewRoom)
	group.GET("/room/find", h.FindRoom)
	group.GET("/room/join", h.JoinRoom)
	return h
}

func (h *RoomsHandler) CreateNewRoom(c echo.Context) error {
	var roomConfig views.CreateRoomRequest
	if err := c.Bind(&roomConfig); err != nil {
		return errz.NewBadRequest("invalid request!")
	}
	if roomConfig.RoomName == "" {
		return errz.NewBadRequest("room name is required!")
	}
	response, err := h.s.GetCreateRoom(c.Request().Context(), roomConfig)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response)
}

func (h *RoomsHandler) FindRoom(c echo.Context) error {
	roomId := c.QueryParam("room_id")
	if roomId == "" {
		return errz.NewBadRequest("room id cannot be empty")
	}
	room, ok := domain.RoomsMap[roomId]
	if !ok {
		return errz.NewNotFound("room not found")
	}
	return c.JSON(http.StatusOK, room)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *RoomsHandler) JoinRoom(c echo.Context) error {
	roomID := c.QueryParam("room_id")
	if roomID == "" {
		return c.String(http.StatusBadRequest, "Missing room 'id'")
	}
	room := domain.GetRoom(roomID)
	if room == nil {
		return errz.NewNotFound("room not found")
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	client := &domain.Client{Conn: ws}

	h.s.Konnection(room, client)
	return nil
}
