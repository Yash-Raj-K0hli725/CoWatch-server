package cogine

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// parseFFmpegProgress reads standard output key-value streams and logs progress percentages
func (c *Cogine) parseFFmpegProgress(r io.Reader, totalDurationSec float64, taskID string) {
	scanner := bufio.NewScanner(r)
	var currentMicroseconds int64

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "out_time_us" {
			if uSec, err := strconv.ParseInt(value, 10, 64); err == nil {
				currentMicroseconds = uSec
				currentSec := float64(currentMicroseconds) / 1000000.0

				if totalDurationSec > 0 {
					pct := (currentSec / totalDurationSec) * 100
					if pct > 100 {
						pct = 100
					}
					fmt.Printf("[%s] CPU Transcoding Progress: %.2f%% (%.1fs / %.1fs)\n", taskID, pct, currentSec, totalDurationSec)
				} else {
					fmt.Printf("[%s] CPU Transcoded: %.1fs\n", taskID, currentSec)
				}
			}
		}
	}
}

// getVideoDurationSeconds retrieves exact media length via ffprobe
func (c *Cogine) getVideoDurationSeconds(inputPath string) (float64, error) {
	ffprobeExe := c.FFProbe
	if ffprobeExe == "" {
		ffprobeExe = "ffprobe"
	}

	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	}

	cmd := exec.Command(ffprobeExe, args...)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	cleanOut := strings.TrimSpace(string(out))
	return strconv.ParseFloat(cleanOut, 64)
}
