package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"resolve/models"
)

// BasicAnalyzer implements the Analyzer interface
type BasicAnalyzer struct {
	id        string
	enabled   bool
	healthy   bool
	mu        sync.RWMutex
	processed int64
}

// NewBasicAnalyzer creates a new basic analyzer
func NewBasicAnalyzer(id string) *BasicAnalyzer {
	return &BasicAnalyzer{
		id:        id,
		enabled:   true,
		healthy:   true,
		processed: 0,
	}
}

// Analyze implements the Analyzer interface
func (a *BasicAnalyzer) Analyze(logMessage models.LogMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.enabled {
		return fmt.Errorf("analyzer %s is disabled", a.id)
	}

	// Simulate analysis processing time
	time.Sleep(10 * time.Millisecond)

	// Print the analyzed message
	fmt.Printf("[%s] Analyzed: %s [%s] %s: %s\n",
		a.id,
		logMessage.Timestamp.Format("15:04:05"),
		logMessage.Level,
		logMessage.Source,
		logMessage.Message)

	// Increment processed count
	a.processed++

	return nil
}

// GetID implements the Analyzer interface
func (a *BasicAnalyzer) GetID() string {
	return a.id
}

// IsHealthy implements the Analyzer interface
func (a *BasicAnalyzer) IsHealthy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.healthy && a.enabled
}

// SetEnabled enables or disables the analyzer
func (a *BasicAnalyzer) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
	fmt.Printf("Analyzer %s %s\n", a.id, map[bool]string{true: "enabled", false: "disabled"}[enabled])
}

// SetHealthy sets the health status of the analyzer
func (a *BasicAnalyzer) SetHealthy(healthy bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.healthy = healthy
	fmt.Printf("Analyzer %s health status: %s\n", a.id, map[bool]string{true: "healthy", false: "unhealthy"}[healthy])
}

// GetProcessedCount returns the number of messages processed by this analyzer
func (a *BasicAnalyzer) GetProcessedCount() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.processed
}

// AnalyzerServer represents an HTTP server that receives and analyzes log messages
type AnalyzerServer struct {
	analyzer *BasicAnalyzer
	port     int
	server   *http.Server
}

// NewAnalyzerServer creates a new analyzer server
func NewAnalyzerServer(analyzer *BasicAnalyzer, port int) *AnalyzerServer {
	return &AnalyzerServer{
		analyzer: analyzer,
		port:     port,
	}
}

// Start starts the HTTP server
func (as *AnalyzerServer) Start() error {
	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/analyze", as.handleAnalyze)
	mux.HandleFunc("/health", as.handleHealth)
	mux.HandleFunc("/status", as.handleStatus)
	mux.HandleFunc("/processed", as.handleProcessed)
	mux.HandleFunc("/disable", as.handleDisable)
	mux.HandleFunc("/enable", as.handleEnable)

	// Create server
	as.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", as.port),
		Handler: mux,
	}

	log.Printf("Analyzer server %s starting on port %d", as.analyzer.GetID(), as.port)
	log.Printf("Health check available at http://localhost:%d/health", as.port)
	log.Printf("Status endpoint available at http://localhost:%d/status", as.port)
	log.Printf("Analyze endpoint available at http://localhost:%d/analyze", as.port)
	log.Printf("Processed count endpoint available at http://localhost:%d/processed", as.port)

	return as.server.ListenAndServe()
}

// Stop gracefully stops the server
func (as *AnalyzerServer) Stop() error {
	if as.server != nil {
		log.Printf("Stopping analyzer server %s", as.analyzer.GetID())
		return as.server.Close()
	}
	return nil
}

// handleAnalyze processes incoming log messages for analysis
func (as *AnalyzerServer) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the log message
	var logMessage models.LogMessage
	if err := json.NewDecoder(r.Body).Decode(&logMessage); err != nil {
		log.Printf("Error decoding log message: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log the received message
	log.Printf("Received log message %s from %s for analysis", logMessage.ID, r.Header.Get("User-Agent"))

	// Analyze the log message
	start := time.Now()
	err := as.analyzer.Analyze(logMessage)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Analysis failed for log message %s: %v (took %v)", logMessage.ID, err, duration)
		http.Error(w, fmt.Sprintf("Analysis failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Log message analyzed successfully",
		"log_id":    logMessage.ID,
		"analyzer":  as.analyzer.GetID(),
		"duration":  duration.String(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
	log.Printf("Successfully analyzed log message %s in %v", logMessage.ID, duration)
}

// handleHealth provides a health check endpoint
func (as *AnalyzerServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := "healthy"
	if !as.analyzer.IsHealthy() {
		status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := map[string]interface{}{
		"status":    status,
		"analyzer":  as.analyzer.GetID(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

// handleStatus provides detailed status information
func (as *AnalyzerServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"analyzer": map[string]interface{}{
			"id":      as.analyzer.GetID(),
			"enabled": as.analyzer.enabled,
			"healthy": as.analyzer.healthy,
		},
		"server": map[string]interface{}{
			"port":    as.port,
			"address": fmt.Sprintf(":%d", as.port),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

// handleProcessed returns the number of messages processed by this analyzer
func (as *AnalyzerServer) handleProcessed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	processedCount := as.analyzer.GetProcessedCount()

	response := map[string]interface{}{
		"analyzer":        as.analyzer.GetID(),
		"processed_count": processedCount,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

// handleDisable disables the analyzer
func (as *AnalyzerServer) handleDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Disable the analyzer
	as.analyzer.SetEnabled(false)

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Analyzer disabled successfully",
		"analyzer":  as.analyzer.GetID(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
	log.Printf("Analyzer %s disabled via HTTP request", as.analyzer.GetID())
}

// handleEnable enables the analyzer
func (as *AnalyzerServer) handleEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Enable the analyzer
	as.analyzer.SetEnabled(true)

	response := map[string]interface{}{
		"status":    "success",
		"message":   "Analyzer enabled successfully",
		"analyzer":  as.analyzer.GetID(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
	log.Printf("Analyzer %s enabled via HTTP request", as.analyzer.GetID())
}

func main() {
	// Default configuration
	analyzerID := "analyzer-1"
	port := 8081

	// Parse command line arguments
	if len(os.Args) > 1 {
		analyzerID = os.Args[1]
	}
	if len(os.Args) > 2 {
		if portArg, err := strconv.Atoi(os.Args[2]); err == nil {
			port = portArg
		}
	}

	// Create analyzer
	analyzer := NewBasicAnalyzer(analyzerID)

	// Create and start analyzer server
	server := NewAnalyzerServer(analyzer, port)

	log.Printf("Starting analyzer server with ID: %s, Port: %d",
		analyzerID, port)

	// Start the server
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start analyzer server: %v", err)
	}
}
