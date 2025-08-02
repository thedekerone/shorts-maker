package models

// Path   – local *finished* .mp4 returned by VeoService
// Length – how long you want the clip to stay on-screen in the final edit
type VideoWithTimestamp struct {
	Path   string  // e.g. /tmp/video_a2Mx3.mp4
	Length float64 // seconds
}
