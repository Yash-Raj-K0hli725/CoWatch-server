package handler

import (
	"StreamRoom/internal/service"
	"net/http"

	"github.com/labstack/echo/v4"
)

type VideoHandler struct {
	s *service.VideoService
	r *service.RoomService
}

func NewVideoHandler(group *echo.Group, videoService *service.VideoService, roomService *service.RoomService) {
	h := &VideoHandler{s: videoService, r: roomService}
	group.GET("/video/upload", h.GetVideoUploadUrl)
}

func (h *VideoHandler) GetVideoUploadUrl(c echo.Context) error {
	roomID := c.QueryParam("room_id")
	url, err := h.s.GenerateUploadUrl(c.Request().Context(), roomID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, url)
}
