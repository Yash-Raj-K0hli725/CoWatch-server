package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

func SanitizeFilename(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9\.]`)
	processed := reg.ReplaceAllString(name, "_")
	return processed
}

// NewID generates a short, collision-resistant identifier for jobs/objects:
// <prefix>_<nanosecond timestamp>_<6 random bytes hex>. The timestamp keeps
// IDs roughly sortable/greppable in logs; the random suffix is what
// actually prevents collisions under concurrent creation.
func NewID(prefix string) string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf) // crypto/rand.Read never errors on a live system
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), hex.EncodeToString(buf))
}

// ParseRoomIDFromObjectKey extracts the room ID out of an upload object key
// of the form "uploads/<roomID>/<file>", which is the layout
// VideoService.GenerateUploadUrl produces. Used by the storage-event
// webhook path, which only receives a bucket + key from the cloud event --
// never a room ID directly.
func ParseRoomIDFromObjectKey(key string) (string, error) {
	parts := strings.Split(key, "/")
	if len(parts) < 3 || parts[0] != "uploads" || parts[1] == "" {
		return "", fmt.Errorf("object key %q is not a recognized upload path", key)
	}
	return parts[1], nil
}
