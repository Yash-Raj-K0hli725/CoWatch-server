package handler

import (
	"StreamRoom/internal/service"
	"fmt"
	"net/http"
	"time"

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
	objectKey := fmt.Sprintf("videos/%s/%s_%d.mp4", roomID, roomID, time.Now().Unix()/1000)
	url, err := h.s.GenerateUploadUrl(c.Request().Context(), objectKey)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, url)
}
