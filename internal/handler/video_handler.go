package handler

import (
	"StreamRoom/errz"
	"StreamRoom/internal/service"
	"StreamRoom/internal/views"
	"net/http"

	"github.com/labstack/echo/v4"
)

type VideoHandler struct {
	s *service.VideoService
	r *service.RoomService
}

func NewVideoHandler(group *echo.Group, videoService *service.VideoService, roomService *service.RoomService, webhookSecret string) {
	h := &VideoHandler{s: videoService, r: roomService}
	group.GET("/video/upload", h.GetVideoUploadUrl)
	group.POST("/video/upload-complete", h.ConfirmUpload)
	group.POST("/webhooks/storage-event", NewStorageWebhookHandler(roomService, webhookSecret))
}

// GetVideoUploadUrl (re)issues a presigned upload URL for an existing room.
// Used for the room's initial upload as well as replacing its video.
func (h *VideoHandler) GetVideoUploadUrl(c echo.Context) error {
	roomID := c.QueryParam("room_id")
	if roomID == "" {
		return errz.NewBadRequest("room_id is required")
	}
	resp, err := h.r.RegenerateUploadUrl(c.Request().Context(), roomID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// ConfirmUpload is called by the client once its presigned PUT to the
// bucket succeeds. The server independently verifies the object exists
// (see VideoService.ConfirmUpload) before enqueueing any work.
func (h *VideoHandler) ConfirmUpload(c echo.Context) error {
	var req views.UploadCompleteRequest
	if err := c.Bind(&req); err != nil {
		return errz.NewBadRequest("invalid request")
	}
	if req.RoomID == "" {
		return errz.NewBadRequest("room_id is required")
	}

	job, err := h.r.ConfirmVideoUpload(c.Request().Context(), req.RoomID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, views.UploadCompleteResponse{JobID: job.JobID, Status: "PROCESSING"})
}
