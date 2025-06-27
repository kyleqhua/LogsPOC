package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"resolve/models"
)

// DistributorServer handles incoming log packets from emitters
type DistributorServer struct {
	config     models.DistributorConfig
	client     *http.Client
	workerPool chan struct{}
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

// // AnalyzerHealth tracks the health status of an analyzer
// type AnalyzerHealth struct {
// 	ID              string
// 	LastSuccess     time.Time
// 	LastFailure     time.Time
// 	FailureCount    int
// 	SuccessCount    int
// 	IsHealthy       bool
// 	LastHealthCheck time.Time
// }

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
	d.distributeLogMessagesParallel(packet.Messages)

	// for _, message := range packet.Messages {
	// 	d.distributeLogMessage(message)
	// }
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
	// Get all analyzers and check their health
	var analyzers []models.AnalyzerConfig
	var totalWeight float64

	for _, analyzer := range d.config.Analyzers {
		// Check if analyzer is healthy by calling its health endpoint
		// if d.isAnalyzerHealthy(analyzer) {
		// 	analyzers = append(healthyAnalyzers, analyzer)
		// 	totalWeight += analyzer.Weight
		// }

		analyzers = append(analyzers, analyzer)
		totalWeight += analyzer.Weight
	}

	// Generate random number between 0 and total weight
	rand.Seed(time.Now().UnixNano())
	randomValue := rand.Float64() * totalWeight

	// Select analyzer based on weighted distribution
	currentWeight := 0.0
	for _, analyzer := range analyzers {
		currentWeight += analyzer.Weight
		if randomValue <= currentWeight {
			return analyzer
		}
	}

	// Fallback to first healthy analyzer (shouldn't reach here)
	return analyzers[0]
}

// // isAnalyzerHealthy checks if an analyzer is healthy by calling its health endpoint
// func (d *DistributorServer) isAnalyzerHealthy(analyzer models.AnalyzerConfig) bool {
// 	client := &http.Client{Timeout: 5 * time.Second}

// 	// Try to hit the health endpoint
// 	healthURL := strings.Replace(analyzer.Endpoint, "/analyze", "/health", 1)
// 	resp, err := client.Get(healthURL)
// 	if err != nil {
// 		log.Printf("Health check failed for analyzer %s: %v", analyzer.ID, err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	return resp.StatusCode == http.StatusOK
// }

// handleHealth provides a health check endpoint
func (d *DistributorServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Distributor is healthy"))
}

// loadConfig loads distributor configuration from a JSON file
func loadConfig(configPath string) (*models.DistributorConfig, error) {
	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse the JSON configuration
	var config models.DistributorConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if config.Port <= 0 {
		return nil, fmt.Errorf("invalid port number: %d", config.Port)
	}

	if len(config.Analyzers) == 0 {
		return nil, fmt.Errorf("no analyzers configured")
	}

	// Calculate total weight
	config.TotalWeight = 0
	for _, analyzer := range config.Analyzers {
		config.TotalWeight += analyzer.Weight
	}

	if config.TotalWeight <= 0 {
		return nil, fmt.Errorf("no analyzers with positive weights")
	}

	log.Printf("Loaded configuration:")
	log.Printf("  Port: %d", config.Port)
	log.Printf("  Total analyzers: %d", len(config.Analyzers))
	log.Printf("  Total weight: %.2f", config.TotalWeight)

	return &config, nil
}

func main() {
	// Load configuration from JSON file
	configPath := "local_config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start the distributor server
	server := NewDistributorServer(*config)

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start distributor server: %v", err)
	}
}
