package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"resolve/models"
)

// DistributorServer handles incoming log packets from emitters
type DistributorServer struct {
	config     models.DistributorConfig
	client     *http.Client
	workerPool chan struct{} // Semaphore to limit concurrent workers
}

// NewDistributorServer creates a new distributor server
func NewDistributorServer(config models.DistributorConfig) *DistributorServer {
	// Create worker pool with reasonable concurrency limit
	maxWorkers := 10 // Adjust based on your needs
	if maxWorkers <= 0 {
		maxWorkers = 10 // Default fallback
	}

	return &DistributorServer{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second, // Default timeout
		},
		workerPool: make(chan struct{}, maxWorkers),
	}
}

// Start starts the HTTP server
func (d *DistributorServer) Start() error {
	// Set up routes
	http.HandleFunc("/logs", d.handleLogPacket)
	http.HandleFunc("/health", d.handleHealth)

	// Start server
	addr := fmt.Sprintf(":%d", d.config.Port)
	log.Printf("Distributor server starting on port %d", d.config.Port)
	log.Printf("Health check available at http://localhost%s/health", addr)
	log.Printf("Log endpoint available at http://localhost%s/logs", addr)

	return http.ListenAndServe(addr, nil)
}

// handleLogPacket processes incoming log packets
func (d *DistributorServer) handleLogPacket(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the log packet
	var packet models.LogPacket
	if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
		log.Printf("Error decoding log packet: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Log packet received successfully"))

	// Process log messages in parallel with backpressure control
	// d.distributeLogMessagesParallel(packet.Messages)

	for _, message := range packet.Messages {
		d.distributeLogMessage(message)
	}
}

// distributeLogMessagesParallel processes multiple log messages concurrently
func (d *DistributorServer) distributeLogMessagesParallel(messages []models.LogMessage) {
	if len(messages) == 0 {
		return
	}

	log.Printf("Processing %d log messages in parallel", len(messages))

	// Use WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup
	wg.Add(len(messages))

	// Process each message in a separate goroutine
	for _, logMessage := range messages {
		go func(msg models.LogMessage) {
			defer wg.Done()

			// Acquire worker slot (backpressure mechanism)
			d.workerPool <- struct{}{}
			defer func() { <-d.workerPool }()

			// Distribute the log message
			d.distributeLogMessage(msg)
		}(logMessage)
	}

	// Wait for all messages to be processed
	wg.Wait()
	log.Printf("Completed processing %d log messages", len(messages))
}

func (d *DistributorServer) distributeLogMessage(logMessage models.LogMessage) {
	// Select analyzer based on weighted distribution
	analyzerConfig := d.selectAnalyzer()
	if analyzerConfig.ID == "" {
		log.Printf("No enabled analyzers available for log message: %s", logMessage.ID)
		return
	}

	log.Printf("Selected analyzer %s (weight: %.2f) for log message: %s",
		analyzerConfig.ID, analyzerConfig.Weight, logMessage.ID)
	// d.printLogMessage(logMessage)

	// Create HTTP client with analyzer-specific timeout
	timeout := time.Duration(analyzerConfig.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 10 * time.Second // Default timeout if not configured
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Serialize log message to JSON
	jsonData, err := json.Marshal(logMessage)
	if err != nil {
		log.Printf("Error marshalling log message %s: %v", logMessage.ID, err)
		return
	}

	// Send request with retries
	var lastErr error
	for attempt := 0; attempt <= analyzerConfig.RetryCount; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying log message %s to analyzer %s (attempt %d/%d)",
				logMessage.ID, analyzerConfig.ID, attempt+1, analyzerConfig.RetryCount+1)
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// Create HTTP request
		req, err := http.NewRequestWithContext(
			ctx,
			"POST",
			analyzerConfig.Endpoint,
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			cancel()
			log.Printf("Error creating request for log message %s: %v", logMessage.ID, err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "log-distributor/1.0")
		req.Header.Set("X-Log-ID", logMessage.ID)
		req.Header.Set("X-Analyzer-ID", analyzerConfig.ID)

		// Send request
		start := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(start)

		cancel() // Always cancel context

		if err != nil {
			lastErr = fmt.Errorf("network error: %w", err)
			log.Printf("Network error sending log message %s to analyzer %s (attempt %d): %v",
				logMessage.ID, analyzerConfig.ID, attempt+1, err)

			if attempt < analyzerConfig.RetryCount {
				// Exponential backoff: 1s, 2s, 4s, 8s...
				backoff := time.Duration(1<<attempt) * time.Second
				log.Printf("Waiting %v before retry for log message %s", backoff, logMessage.ID)
				time.Sleep(backoff)
			}
			continue
		}

		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode == http.StatusOK {
			log.Printf("Successfully sent log message %s to analyzer %s in %v",
				logMessage.ID, analyzerConfig.ID, duration)
			return
		}

		// Handle non-200 status codes
		lastErr = fmt.Errorf("analyzer returned status code: %d", resp.StatusCode)
		log.Printf("Analyzer %s returned status code %d for log message %s (attempt %d)",
			analyzerConfig.ID, resp.StatusCode, logMessage.ID, attempt+1)

		// Don't retry on client errors (4xx) unless it's a 429 (rate limit)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			log.Printf("Not retrying log message %s due to client error (status %d)",
				logMessage.ID, resp.StatusCode)
			return
		}

		if attempt < analyzerConfig.RetryCount {
			// Exponential backoff for server errors
			backoff := time.Duration(1<<attempt) * time.Second
			log.Printf("Waiting %v before retry for log message %s", backoff, logMessage.ID)
			time.Sleep(backoff)
		}
	}

	// All retries exhausted
	log.Printf("Failed to send log message %s to analyzer %s after %d attempts. Last error: %v",
		logMessage.ID, analyzerConfig.ID, analyzerConfig.RetryCount+1, lastErr)
}

func (d *DistributorServer) selectAnalyzer() models.AnalyzerConfig {
	// Filter enabled analyzers
	var enabledAnalyzers []models.AnalyzerConfig
	var totalWeight float64

	for _, analyzer := range d.config.Analyzers {
		if analyzer.Enabled {
			enabledAnalyzers = append(enabledAnalyzers, analyzer)
			totalWeight += analyzer.Weight
		}
	}

	if len(enabledAnalyzers) == 0 {
		return models.AnalyzerConfig{}
	}

	// Generate random number between 0 and total weight
	rand.Seed(time.Now().UnixNano())
	randomValue := rand.Float64() * totalWeight

	// Select analyzer based on weighted distribution
	currentWeight := 0.0
	for _, analyzer := range enabledAnalyzers {
		currentWeight += analyzer.Weight
		if randomValue <= currentWeight {
			return analyzer
		}
	}

	// Fallback to first enabled analyzer (shouldn't reach here)
	return enabledAnalyzers[0]
}

// handleHealth provides a health check endpoint
func (d *DistributorServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Distributor is healthy"))
}

func (d *DistributorServer) printLogMessage(message models.LogMessage) {
	fmt.Printf("\n=== LOG MESSAGE ===\n")
	fmt.Printf("Message ID: %s\n", message.ID)
	fmt.Printf("Timestamp: %s\n", message.Timestamp.Format(time.RFC3339))
	fmt.Printf("Level: %s\n", message.Level)
	fmt.Printf("Source: %s\n", message.Source)
}

func main() {
	// Create and start the distributor server
	server := NewDistributorServer(models.DistributorConfig{
		Port: 8080,
		Analyzers: []models.AnalyzerConfig{
			{
				ID:         "analyzer-1",
				Weight:     1,
				Enabled:    true,
				Endpoint:   "http://localhost:8081/analyze",
				Timeout:    10000,
				RetryCount: 3,
			},
		},
	})

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start distributor server: %v", err)
	}
}
