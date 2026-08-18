package handler

import (
	"StreamRoom/errz"
	"StreamRoom/internal/service"
	"StreamRoom/internal/views"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// NewStorageWebhookHandler returns an echo.HandlerFunc ingesting a
// normalized "object created" event from the storage layer (S3/R2).
//
// Native cloud event shapes differ (S3 -> EventBridge/SNS, R2 -> Event
// Notifications -> Queue), so a thin relay (a Lambda or a Cloudflare
// Worker) is expected to translate the native event into the
// views.StorageEventPayload contract and HMAC-sign the raw body with the
// shared webhook secret before forwarding it here. If webhookSecret is
// empty, signature verification is skipped -- fine for local dev, never
// for a publicly reachable deployment.
func NewStorageWebhookHandler(roomService *service.RoomService, webhookSecret string) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return errz.NewBadRequest("could not read request body")
		}

		if webhookSecret != "" {
			sig := c.Request().Header.Get("X-StreamRoom-Signature")
			if !verifyHMAC(webhookSecret, body, sig) {
				return errz.NewUnauthorized("invalid webhook signature")
			}
		}

		var evt views.StorageEventPayload
		if err := json.Unmarshal(body, &evt); err != nil {
			return errz.NewBadRequest("invalid event payload")
		}
		if evt.ObjectKey == "" {
			return errz.NewBadRequest("object_key is required")
		}
		// Only object-created events represent a finished upload; other
		// event types are acknowledged (not errored) so a relay can safely
		// forward everything and let us filter.
		if evt.EventType != "" && evt.EventType != "ObjectCreated" {
			return c.JSON(http.StatusOK, views.Response{Message: "ignored"})
		}

		job, err := roomService.IngestVideoUploadEvent(c.Request().Context(), evt.Bucket, evt.ObjectKey, evt.SizeBytes, evt.ETag)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusAccepted, views.UploadCompleteResponse{JobID: job.JobID, Status: "PROCESSING"})
	}
}

func verifyHMAC(secret string, body []byte, sigHeader string) bool {
	if sigHeader == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}
