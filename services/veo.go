package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
)

// VeoService lets you talk to Vertex-AI Veo models.
type VeoService struct {
	ProjectID string
	Location  string
	ModelID   string
}

// NewVeoService returns a new instance.
func NewVeoService(projectID, location, modelID string) (*VeoService, error) {
	if projectID == "" || location == "" || modelID == "" {
		return nil, errors.New("projectID, location and modelID are required")
	}
	return &VeoService{ProjectID: projectID, Location: location, ModelID: modelID}, nil
}

// StartGeneration submits the long-running Veo request and returns its operation name.
func (vs *VeoService) StartGeneration(prompt string, duration int, resolution string, sampleCount int) (string, error) {
	ctx := context.Background()
	if duration <= 0 {
		duration = 8
	}
	if sampleCount <= 0 {
		sampleCount = 1
	}

	// Build JSON body.
	params := map[string]interface{}{
		"aspectRatio":      "9:16",
		"durationSeconds":  duration,
		"sampleCount":      sampleCount,
		"personGeneration": "allow_adult",
		"addWatermark":     true,
		"includeRaiReason": true,
		"generateAudio":    false,
	}
	// “resolution” only for Veo-3 models.
	if resolution != "" && strings.HasPrefix(vs.ModelID, "veo-3") {
		params["resolution"] = resolution
	}
	reqBody := map[string]interface{}{
		"instances":  []map[string]string{{"prompt": prompt}},
		"parameters": params,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Auth token.
	token, err := vs.getAccessToken(ctx)
	if err != nil {
		return "", err
	}

	url := "https://" + vs.Location + "-aiplatform.googleapis.com/v1/projects/" +
		vs.ProjectID + "/locations/" + vs.Location + "/publishers/google/models/" +
		vs.ModelID + ":predictLongRunning"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	println(url)

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", errors.New("predictLongRunning failed: " + string(b))
	}

	var op struct{ Name string }
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return "", err
	}
	if op.Name == "" {
		return "", errors.New("operation name missing in response")
	}
	println(op.Name)
	return op.Name, nil
}

// PollGeneration waits for the long-running operation to finish,
// downloads video(s) (Base64 or GCS) and returns local temp-file paths.
func (vs *VeoService) PollGeneration(operationName string) ([]string, error) {
	ctx := context.Background()
	token, err := vs.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	fetchURL := "https://" + vs.Location + "-aiplatform.googleapis.com/v1/projects/" +
		vs.ProjectID + "/locations/" + vs.Location + "/publishers/google/models/" +
		vs.ModelID + ":fetchPredictOperation"
	reqBody, _ := json.Marshal(map[string]string{"operationName": operationName})

	var paths []string
	for attempts := 0; attempts < 120; attempts++ { // ~10 min max
		time.Sleep(5 * time.Second)

		req, _ := http.NewRequest("POST", fetchURL, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		fmt.Printf("FETCH COUNT %d", attempts)

		resp, err := new(http.Client).Do(req)
		if err != nil {
			println(err.Error())
			return nil, err
		}
		var res struct {
			Done     bool   `json:"done"`
			Name     string `json:"name"`
			Response struct {
				Videos []struct {
					Bytes string `json:"bytesBase64Encoded"`
					URI   string `json:"gcsUri"`
				} `json:"videos"`
			} `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		print("not done yet")
		if !res.Done {
			continue
		}

		fmt.Print("%v", res)
		if len(res.Response.Videos) == 0 {
			continue
		}

		println("DONE")
		// Done – process videos.
		for _, v := range res.Response.Videos {
			data, err := vs.fetchVideoBytes(ctx, v.Bytes, v.URI)
			if err != nil {
				return nil, err
			}
			tmp, err := os.CreateTemp("", "video_*.mp4")
			if err != nil {
				return nil, err
			}
			if _, err := tmp.Write(data); err != nil {
				tmp.Close()
				return nil, err
			}
			tmp.Close()
			paths = append(paths, tmp.Name())
		}
		break
	}

	if len(paths) == 0 {
		return nil, errors.New("operation finished but no videos returned")
	}
	return paths, nil
}

// GetVideos = StartGeneration + PollGeneration (kept for convenience).
func (vs *VeoService) GetVideos(prompt string, duration int, resolution string, sampleCount int) ([]string, error) {
	op, err := vs.StartGeneration(prompt, duration, resolution, sampleCount)
	if err != nil {
		return nil, err
	}
	return vs.PollGeneration(op)
}

// -------- helpers --------

func (vs *VeoService) getAccessToken(ctx context.Context) (string, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", err
	}
	tok, err := creds.TokenSource.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (vs *VeoService) fetchVideoBytes(ctx context.Context, b64, gcsURI string) ([]byte, error) {
	switch {
	case b64 != "":
		return base64.StdEncoding.DecodeString(b64)
	case gcsURI != "":
		return downloadFromGCS(ctx, gcsURI)
	default:
		return nil, errors.New("video payload empty")
	}
}

func downloadFromGCS(ctx context.Context, uri string) ([]byte, error) {
	if !strings.HasPrefix(uri, "gs://") {
		return nil, errors.New("invalid gcs uri")
	}
	trim := strings.TrimPrefix(uri, "gs://")
	parts := strings.SplitN(trim, "/", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed gcs uri")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	rc, err := client.Bucket(parts[0]).Object(parts[1]).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
