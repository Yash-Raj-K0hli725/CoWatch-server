package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeMasterPlaylist writes master.m3u8 into outputDir, listing exactly
// the renditions that were actually produced (never a hard-coded list --
// low-res sources only get the rungs that fit under their native
// resolution, see ladderFor).
func writeMasterPlaylist(outputDir string, renditions []rendition) (string, error) {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	for _, r := range renditions {
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s.m3u8\n", r.Bandwidth, r.Width, r.Height, r.Name)
	}

	masterPath := filepath.Join(outputDir, "master.m3u8")
	if err := os.WriteFile(masterPath, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write master playlist: %w", err)
	}
	return masterPath, nil
}
