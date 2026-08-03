package control

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxVideoFrames = 6

// isVideoPath identifies video files that should be represented by sampled
// frames instead of being read as binary text.
func isVideoPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".m4v", ".mkv", ".webm", ".avi", ".wmv", ".flv":
		return true
	default:
		return false
	}
}

func videoRefNote(path string, size int64) string {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Sprintf("[video file %s, %d bytes; install ffmpeg to extract keyframes for visual analysis]", path, size)
	}
	return fmt.Sprintf("[video file %s, %d bytes; Vcode will sample up to %d keyframes for visual analysis]", path, size, maxVideoFrames)
}

// videoFrameDataURLs samples a local video with ffmpeg and returns compressed
// image data URLs. Keeping the video itself out of the request makes this work
// with providers that accept images but do not expose a native video input.
func videoFrameDataURLs(path, baseDir string) ([]string, error) {
	absPath, _, ok := resolveAbsRef(path, baseDir)
	if !ok {
		return nil, os.ErrNotExist
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg is required to read video %q", path)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("video path %q is a directory", path)
	}
	tmp, err := os.MkdirTemp("", "vcode-video-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	pattern := filepath.Join(tmp, "frame-%02d.jpg")
	cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-i", absPath,
		"-vf", "fps=1/5,scale=1280:-2:force_original_aspect_ratio=decrease",
		"-frames:v", fmt.Sprintf("%d", maxVideoFrames), "-q:v", "3", "-y", pattern)
	if output, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("ffmpeg video frame extraction failed: %s", msg)
	}
	frames, err := filepath.Glob(filepath.Join(tmp, "frame-*.jpg"))
	if err != nil || len(frames) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no video frames")
	}
	var urls []string
	for _, frame := range frames {
		raw, err := os.ReadFile(frame)
		if err != nil {
			continue
		}
		raw, mime := compressForVision(raw, "image/jpeg")
		urls = append(urls, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(raw))
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("video frames could not be read")
	}
	return urls, nil
}
