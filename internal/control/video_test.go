package control

import "testing"

func TestIsVideoPath(t *testing.T) {
	for _, path := range []string{"movie.mp4", "clip.MOV", "recording.webm", "screen.mkv"} {
		if !isVideoPath(path) {
			t.Errorf("isVideoPath(%q) = false", path)
		}
	}
	for _, path := range []string{"photo.png", "report.pdf", "notes.txt"} {
		if isVideoPath(path) {
			t.Errorf("isVideoPath(%q) = true", path)
		}
	}
}
