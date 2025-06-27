package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"resolve/emitters"
	"resolve/models"
)

// EmitterServer simulates a web application that generates logs and sends them to distributors
type EmitterServer struct {
	config      EmitterServerConfig
	emitterPool *emitters.EmitterPoolImpl
	mu          sync.RWMutex
	stats       EmitterServerStats
}

// EmitterServerConfig holds the configuration for the emitter server
type EmitterServerConfig struct {
	Port              int      `json:"port"`
	DistributorURLs   []string `json:"distributor_urls"`
	LogGenerationRate int      `json:"log_generation_rate"` // logs per second
	MaxConcurrency    int      `json:"max_concurrency"`
	BatchSize         int      `json:"batch_size"`
	FlushInterval     int      `json:"flush_interval"` // milliseconds
}

// EmitterServerStats tracks the performance of the emitter server
type EmitterServerStats struct {
	LogsGenerated    int64     `json:"logs_generated"`
	PacketsSent      int64     `json:"packets_sent"`
	SuccessfulSends  int64     `json:"successful_sends"`
	FailedSends      int64     `json:"failed_sends"`
	StartTime        time.Time `json:"start_time"`
	LastActivityTime time.Time `json:"last_activity_time"`
}

// NewEmitterServer creates a new emitter server
func NewEmitterServer(config EmitterServerConfig) *EmitterServer {
	return &EmitterServer{
		config:      config,
		emitterPool: emitters.NewEmitterPool(),
		stats: EmitterServerStats{
			StartTime: time.Now(),
		},
	}
}

// Start starts the emitter server
func (em *EmitterServer) Start() error {
	// Initialize emitters for each distributor
	if err := em.initializeEmitters(); err != nil {
		return fmt.Errorf("failed to initialize emitters: %w", err)
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", em.handleHealth)
	mux.HandleFunc("/stats", em.handleStats)
	mux.HandleFunc("/start", em.handleStart)
	mux.HandleFunc("/stop", em.handleStop)
	mux.HandleFunc("/generate", em.handleGenerateLogs)

	// Create server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", em.config.Port),
		Handler: mux,
	}

	log.Printf("Emitter server starting on port %d", em.config.Port)
	log.Printf("Health check available at http://localhost:%d/health", em.config.Port)
	log.Printf("Stats endpoint available at http://localhost:%d/stats", em.config.Port)
	log.Printf("Start log generation: http://localhost:%d/start", em.config.Port)
	log.Printf("Stop log generation: http://localhost:%d/stop", em.config.Port)
	log.Printf("Generate single batch: http://localhost:%d/generate", em.config.Port)

	return server.ListenAndServe()
}

// initializeEmitters creates HTTP emitters for each distributor URL and adds them to the pool
func (em *EmitterServer) initializeEmitters() error {
	for i, distributorURL := range em.config.DistributorURLs {
		emitterID := fmt.Sprintf("emitter-%d", i+1)

		emitterConfig := models.EmitterConfig{
			ID:             emitterID,
			Endpoint:       distributorURL,
			Timeout:        30 * time.Second,
			RetryCount:     3,
			RetryDelay:     1 * time.Second,
			MaxConcurrency: em.config.MaxConcurrency,
			BufferSize:     1000,
			BatchSize:      em.config.BatchSize,
			FlushInterval:  time.Duration(em.config.FlushInterval) * time.Millisecond,
		}

		emitter := emitters.NewHTTPEmitter(emitterConfig)

		// Add emitter to the pool
		if err := em.emitterPool.AddEmitter(emitter); err != nil {
			return fmt.Errorf("failed to add emitter %s to pool: %w", emitterID, err)
		}

		log.Printf("Initialized emitter %s for distributor %s", emitterID, distributorURL)
	}

	log.Printf("Initialized emitter pool with %d emitters", em.emitterPool.GetEmitterCount())
	return nil
}

// generateLogs creates a batch of log messages
func (em *EmitterServer) generateLogs() models.LogPacket {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Generate log messages
	messages := make([]models.LogMessage, em.config.BatchSize)
	timestamp := time.Now()
	packetID := fmt.Sprintf("packet-%d", em.stats.PacketsSent+1)

	for i := 0; i < em.config.BatchSize; i++ {
		// Generate random log data
		logLevel := em.getRandomLogLevel()
		source := em.getRandomSource()
		message := em.getRandomMessage(logLevel, source)

		messages[i] = models.LogMessage{
			ID:        fmt.Sprintf("log-%s-%d", packetID, i+1),
			Timestamp: timestamp.Add(time.Duration(i) * time.Millisecond),
			Level:     logLevel,
			Source:    source,
			Message:   message,
			Metadata: map[string]string{
				"user_id":    fmt.Sprintf("user-%d", rand.Intn(1000)),
				"session_id": fmt.Sprintf("session-%d", rand.Intn(10000)),
				"ip":         fmt.Sprintf("192.168.1.%d", rand.Intn(255)),
			},
		}
	}

	// Create log packet
	packet := models.LogPacket{
		PacketID:  packetID,
		AgentID:   "emitter-server",
		Timestamp: timestamp,
		Messages:  messages,
	}

	// Update stats
	em.stats.LogsGenerated += int64(len(messages))
	em.stats.PacketsSent++
	em.stats.LastActivityTime = time.Now()

	return packet
}

// getRandomLogLevel returns a random log level with weighted distribution
func (em *EmitterServer) getRandomLogLevel() string {
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	weights := []int{20, 60, 15, 5} // 20% DEBUG, 60% INFO, 15% WARN, 5% ERROR

	total := 0
	for _, weight := range weights {
		total += weight
	}

	random := rand.Intn(total)
	current := 0

	for i, weight := range weights {
		current += weight
		if random < current {
			return levels[i]
		}
	}

	return "INFO" // fallback
}

// getRandomSource returns a random source component
func (em *EmitterServer) getRandomSource() string {
	sources := []string{
		"web-server",
		"database",
		"auth-service",
		"payment-service",
		"user-service",
		"notification-service",
		"api-gateway",
		"cache-service",
	}
	return sources[rand.Intn(len(sources))]
}

// getRandomMessage generates a random log message based on level and source
func (em *EmitterServer) getRandomMessage(level, source string) string {
	messages := map[string][]string{
		"DEBUG": {
			"Processing request",
			"Database query executed",
			"Cache hit",
			"Validation passed",
			"Connection established",
		},
		"INFO": {
			"User login successful",
			"Payment processed",
			"Email sent",
			"File uploaded",
			"Data synchronized",
		},
		"WARN": {
			"High memory usage detected",
			"Slow database query",
			"Cache miss",
			"Retry attempt",
			"Deprecated API called",
		},
		"ERROR": {
			"Database connection failed",
			"Authentication failed",
			"Payment declined",
			"File not found",
			"Network timeout",
		},
	}

	levelMessages := messages[level]
	if len(levelMessages) == 0 {
		return fmt.Sprintf("Unknown log level: %s", level)
	}

	return fmt.Sprintf("[%s] %s", source, levelMessages[rand.Intn(len(levelMessages))])
}

// sendLogs sends log packets to all distributors using the emitter pool
func (em *EmitterServer) sendLogs(packet models.LogPacket) {
	// Get all emitters from the pool
	allEmitters := em.emitterPool.GetAllEmitters()

	var wg sync.WaitGroup
	wg.Add(len(allEmitters))

	for emitterID, emitter := range allEmitters {
		go func(id string, e models.Emitter) {
			defer wg.Done()

			if err := e.Emit(packet); err != nil {
				log.Printf("Emitter %s failed to send packet %s: %v", id, packet.PacketID, err)
				em.mu.Lock()
				em.stats.FailedSends++
				em.mu.Unlock()
			} else {
				log.Printf("Emitter %s successfully sent packet %s with %d messages",
					id, packet.PacketID, len(packet.Messages))
				em.mu.Lock()
				em.stats.SuccessfulSends++
				em.mu.Unlock()
			}
		}(emitterID, emitter)
	}

	wg.Wait()
}

// HTTP handlers

func (em *EmitterServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "emitter-server",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (em *EmitterServer) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	em.mu.RLock()
	stats := em.stats
	em.mu.RUnlock()

	response := map[string]interface{}{
		"stats": map[string]interface{}{
			"logs_generated":     stats.LogsGenerated,
			"packets_sent":       stats.PacketsSent,
			"successful_sends":   stats.SuccessfulSends,
			"failed_sends":       stats.FailedSends,
			"start_time":         stats.StartTime.Format(time.RFC3339),
			"last_activity_time": stats.LastActivityTime.Format(time.RFC3339),
			"uptime":             time.Since(stats.StartTime).String(),
		},
		"config": map[string]interface{}{
			"port":                em.config.Port,
			"log_generation_rate": em.config.LogGenerationRate,
			"max_concurrency":     em.config.MaxConcurrency,
			"batch_size":          em.config.BatchSize,
			"flush_interval":      em.config.FlushInterval,
			"distributor_count":   len(em.config.DistributorURLs),
		},
		"emitter_pool": map[string]interface{}{
			"count": em.emitterPool.GetEmitterCount(),
			"ids": func() []string {
				allEmitters := em.emitterPool.GetAllEmitters()
				ids := make([]string, 0, len(allEmitters))
				for id := range allEmitters {
					ids = append(ids, id)
				}
				return ids
			}(),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (em *EmitterServer) handleStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Start continuous log generation in background
	go em.startContinuousGeneration()

	response := map[string]interface{}{
		"status":    "started",
		"message":   "Log generation started",
		"rate":      fmt.Sprintf("%d logs/second", em.config.LogGenerationRate),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (em *EmitterServer) handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Stop continuous generation (implement with context cancellation if needed)
	response := map[string]interface{}{
		"status":    "stopped",
		"message":   "Log generation stopped",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (em *EmitterServer) handleGenerateLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Generate and send a single batch of logs
	packet := em.generateLogs()
	em.sendLogs(packet)

	response := map[string]interface{}{
		"status":    "generated",
		"message":   "Log batch generated and sent",
		"packet_id": packet.PacketID,
		"messages":  len(packet.Messages),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

// startContinuousGeneration starts continuous log generation
func (em *EmitterServer) startContinuousGeneration() {
	ticker := time.NewTicker(time.Duration(1000/em.config.LogGenerationRate) * time.Millisecond)
	defer ticker.Stop()

	log.Printf("Starting continuous log generation at %d logs/second", em.config.LogGenerationRate)

	for range ticker.C {
		packet := em.generateLogs()
		em.sendLogs(packet)
	}
}

func main() {
	// Default configuration
	config := EmitterServerConfig{
		Port:              8084,
		DistributorURLs:   []string{"http://localhost:8080/logs"},
		LogGenerationRate: 10, // 10 logs per second
		MaxConcurrency:    5,
		BatchSize:         5,    // 5 messages per packet
		FlushInterval:     1000, // 1 second
	}

	// Create and start emitter server
	server := NewEmitterServer(config)

	log.Printf("Starting emitter server with configuration:")
	log.Printf("  Port: %d", config.Port)
	log.Printf("  Distributors: %v", config.DistributorURLs)
	log.Printf("  Log generation rate: %d logs/second", config.LogGenerationRate)
	log.Printf("  Batch size: %d", config.BatchSize)
	log.Printf("  Max concurrency: %d", config.MaxConcurrency)

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start emitter server: %v", err)
	}
}
