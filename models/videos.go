// models/video.go  (new file – name/location up to you)
package models

type VideoWithTimestamp struct {
	Path      string  // local tmp file (or GCS uri if you keep it remote)
	Timestamp float64 // seconds to keep clip on-screen when you edit
}
