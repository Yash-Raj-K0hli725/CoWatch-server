package worker

import "fmt"

// rendition describes one output of the adaptive-bitrate HLS ladder.
type rendition struct {
	Name             string // "1080p" -- also the segment/playlist filename stem
	Width            int
	Height           int
	VideoBitrateKbps int
	MaxrateKbps      int
	BufsizeKbps      int
	AudioBitrateKbps int
	// Bandwidth is the approximate total bits/sec advertised in the master
	// playlist's BANDWIDTH attribute (video + audio bitrate, in bits/sec).
	Bandwidth int
}

// fullLadder is the full quality ladder, highest first. ladderFor trims it
// down to whatever doesn't exceed the source's resolution -- we never
// manufacture detail that isn't in the source.
var fullLadder = []rendition{
	{Name: "1080p", Width: 1920, Height: 1080, VideoBitrateKbps: 5000, MaxrateKbps: 5350, BufsizeKbps: 7500, AudioBitrateKbps: 192, Bandwidth: 5300000},
	{Name: "720p", Width: 1280, Height: 720, VideoBitrateKbps: 2800, MaxrateKbps: 2996, BufsizeKbps: 4200, AudioBitrateKbps: 128, Bandwidth: 3000000},
	{Name: "480p", Width: 854, Height: 480, VideoBitrateKbps: 1400, MaxrateKbps: 1498, BufsizeKbps: 2100, AudioBitrateKbps: 96, Bandwidth: 1500000},
	{Name: "360p", Width: 640, Height: 360, VideoBitrateKbps: 800, MaxrateKbps: 856, BufsizeKbps: 1200, AudioBitrateKbps: 96, Bandwidth: 900000},
}

// ladderFor returns the renditions from fullLadder that don't exceed the
// source's height, so a 480p upload never gets upscaled to a fake 1080p
// rendition. If the source is smaller than even the lowest rung, a single
// rendition is synthesized at (roughly) the source's native size instead of
// refusing to transcode it at all.
func ladderFor(srcWidth, srcHeight int) []rendition {
	var out []rendition
	for _, r := range fullLadder {
		if r.Height <= srcHeight {
			out = append(out, r)
		}
	}
	if len(out) > 0 {
		return out
	}
	if srcHeight <= 0 || srcWidth <= 0 {
		return nil
	}

	fallback := fullLadder[len(fullLadder)-1]
	h := evenInt(srcHeight)
	w := evenInt(srcWidth)
	fallback.Width, fallback.Height = w, h
	fallback.Name = fmt.Sprintf("%dp", h)
	return []rendition{fallback}
}

// evenInt rounds down to the nearest even number -- required for yuv420p
// output, which most H.264 players (and HLS itself) assume.
func evenInt(n int) int {
	if n%2 != 0 {
		return n - 1
	}
	return n
}
