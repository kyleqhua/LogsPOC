package emitters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"resolve/models"
)

// HTTPEmitter implements the Emitter interface for sending log packets via HTTP
type HTTPEmitter struct {
	id       string
	endpoint string
	client   *http.Client
	config   models.EmitterConfig
}

// NewHTTPEmitter creates a new HTTP emitter
func NewHTTPEmitter(config models.EmitterConfig) *HTTPEmitter {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	emitter := &HTTPEmitter{
		id:       config.ID,
		endpoint: config.Endpoint,
		client:   client,
		config:   config,
	}

	return emitter
}

// Emit sends a log packet to the distributor
func (e *HTTPEmitter) Emit(packet models.LogPacket) error {
	// Serialize packet to JSON
	jsonData, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("failed to marshal packet: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		e.endpoint,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "log-emitter/1.0")

	// Send request with retries
	var resp *http.Response
	for attempt := 0; attempt <= e.config.RetryCount; attempt++ {
		resp, err = e.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if attempt < e.config.RetryCount {
			time.Sleep(e.config.RetryDelay)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to send request after %d attempts: %w", e.config.RetryCount+1, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("distributor returned status code: %d", resp.StatusCode)
	}

	return nil
}

// GetID returns the emitter ID
func (e *HTTPEmitter) GetID() string {
	return e.id
}

// GetEndpoint returns the emitter endpoint
func (e *HTTPEmitter) GetEndpoint() string {
	return e.endpoint
}
