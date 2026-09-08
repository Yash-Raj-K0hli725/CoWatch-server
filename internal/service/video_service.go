package service

import (
	"StreamRoom/storage"
	"context"
)

type VideoService struct {
	r2 *storage.R2MediaService
}

func NewVideoService(r2 *storage.R2MediaService) *VideoService {
	return &VideoService{r2: r2}
}

// UploadToStorage simulates saving to S3/CDN or local disk
//	func (s *VideoService) UploadToStorage(file io.Reader, filename string) (string, error) {
//		// Clean the filename to prevent directory traversal attacks
//		cleanName := filepath.Base(filename)
//		cleanName = util.SanitizeFilename(cleanName)
//		uniqueId := fmt.Sprintf("%d_%s", time.Now().Unix(), cleanName)
//		outputDir := filepath.Join("C:/co-watch/videos", uniqueId)
//		_ = os.MkdirAll(outputDir, os.ModePerm)
//
//		rawPath := filepath.Join(outputDir, "raw_"+cleanName)
//		dst, err := os.Create(rawPath)
//		if err != nil {
//			return "", err
//		}
//		defer dst.Close()
//
//		// Stream the data chunks smoothly from the upload body into the destination file
//		if _, err = io.Copy(dst, file); err != nil {
//			return "", err
//		}
//		//masterPlaylistPath := filepath.Join(outputDir, "master.m3u8")//TODO ????
//		go s.transcodeToHLSWithGPU(rawPath, outputDir)
//		// In real life, you would push this file to AWS S3 / Cloudflare R2 and get a CDN URL.
//		// For now, we return a mock URL pointing to our media asset pipeline.
//		mockCDNURL := fmt.Sprintf("https://cdn.myapp.com/storage/%s/master.m3u8", uniqueId)
//		return mockCDNURL, nil
//	}
//
// // transcodeToHLSWithGPU uses NVIDIA NVENC GPU acceleration to scale and packetize video into HLS chunks
//
//	func (s *VideoService) transcodeToHLSWithGPU(inputPath, outputDir string) {
//		defer os.Remove(inputPath)
//		ffmpegInstance := "C:\\ffmpeg\\bin\\ffmpeg.exe"
//		args := []string{
//			"-i", inputPath, // CPU decodes the source cleanly
//			"-hide_banner", "-y",
//
//			// ----------------------------------------------------
//			// 720p Variant (Scale via CPU -> Encode via RTX GPU)
//			// ----------------------------------------------------
//			"-vf", "scale=1280:720", // Standard stable filter scaling
//			"-c:a", "aac", "-ar", "48000", "-b:a", "128k",
//			"-c:v", "h264_nvenc", // Hand off immediately to RTX 2060 Super
//			"-preset", "p4",
//			"-g", "60", "-keyint_min", "60",
//			"-b:v", "2500k",
//			"-hls_time", "2",
//			"-hls_playlist_type", "vod",
//			"-hls_segment_filename", filepath.Join(outputDir, "720p_%03d.ts"),
//			filepath.Join(outputDir, "720p.m3u8"),
//
//			// ----------------------------------------------------
//			// 480p Variant (Scale via CPU -> Encode via RTX GPU)
//			// ----------------------------------------------------
//			"-vf", "scale=854:480", // Standard stable filter scaling
//			"-c:a", "aac", "-ar", "48000", "-b:a", "96k",
//			"-c:v", "h264_nvenc", // Hand off immediately to RTX 2060 Super
//			"-preset", "p4",
//			"-g", "60", "-keyint_min", "60",
//			"-b:v", "1000k",
//			"-hls_time", "2",
//			"-hls_playlist_type", "vod",
//			"-hls_segment_filename", filepath.Join(outputDir, "480p_%03d.ts"),
//			filepath.Join(outputDir, "480p.m3u8"),
//		}
//
//		cmd := exec.Command(ffmpegInstance, args...)
//		cmd.Stderr = os.Stderr
//		cmd.Stdout = os.Stdout
//
//		// Execute the command
//		if err := cmd.Run(); err != nil {
//			fmt.Printf("❌ GPU Transcoding error processing %s: %v\n", inputPath, err)
//			return
//		}
//
//		// Create the Master Playlist file that tells ExoPlayer which configurations exist
//		s.createMasterPlaylist(outputDir)
//		fmt.Printf("🚀 GPU Transcoding successfully completed for directory: %s\n", outputDir)
//	}

func (s *VideoService) GenerateUploadUrl(c context.Context, obzectKey string) (string, error) {
	return s.r2.GenerateUploadURL(c, "video/mp4", obzectKey)
}
