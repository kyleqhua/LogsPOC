package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"resolve/emitters"
	"resolve/models"
)

func main() {
	// Check command line arguments
	if len(os.Args) > 1 && os.Args[1] == "multi" {
		runMultiEmitterTest()
		return
	}

	// Default: run single emitter test
	runSingleEmitterTest()
}

func runSingleEmitterTest() {
	// Create emitter configuration
	config := models.EmitterConfig{
		ID:            "test-agent-1",
		Endpoint:      "http://localhost:8080/logs",
		Timeout:       10 * time.Second,
		RetryCount:    3,
		RetryDelay:    1 * time.Second,
		BatchSize:     5,
		FlushInterval: 2 * time.Second,
	}

	// Create HTTP emitter
	emitter := emitters.NewHTTPEmitter(config)

	// Create sample log messages
	sampleMessages := []models.LogMessage{
		{
			ID:        "msg-1",
			Timestamp: time.Now(),
			Level:     "INFO",
			Source:    "test-app",
			Message:   "Application started successfully",
			Metadata: map[string]string{
				"version": "1.0.0",
				"env":     "development",
			},
		},
		{
			ID:        "msg-2",
			Timestamp: time.Now().Add(time.Second),
			Level:     "DEBUG",
			Source:    "test-app",
			Message:   "Processing user request",
			Metadata: map[string]string{
				"user_id": "12345",
				"action":  "login",
			},
		},
		{
			ID:        "msg-3",
			Timestamp: time.Now().Add(2 * time.Second),
			Level:     "WARN",
			Source:    "test-app",
			Message:   "Database connection slow",
			Metadata: map[string]string{
				"db_host": "localhost",
				"latency": "150ms",
			},
		},
	}

	// Create log packet
	packet := models.LogPacket{
		PacketID:  fmt.Sprintf("packet-%d", time.Now().Unix()),
		AgentID:   config.ID,
		Timestamp: time.Now(),
		Messages:  sampleMessages,
	}

	// Send the packet
	log.Printf("Sending log packet to %s", config.Endpoint)

	if err := emitter.Emit(packet); err != nil {
		log.Printf("Error sending packet: %v", err)
	} else {
		log.Printf("Packet sent successfully!")
	}

	// Print the packet that was sent (for verification)
	fmt.Printf("\n=== SENT PACKET ===\n")
	jsonData, _ := json.MarshalIndent(packet, "", "  ")
	fmt.Println(string(jsonData))
	fmt.Printf("=== END SENT PACKET ===\n")
}

// EmitterSimulator simulates a single emitter sending packets
type EmitterSimulator struct {
	ID       string
	Config   models.EmitterConfig
	Emitter  models.Emitter
	Packets  int
	Interval time.Duration
}

// NewEmitterSimulator creates a new emitter simulator
func NewEmitterSimulator(id string, packets int, interval time.Duration) *EmitterSimulator {
	config := models.EmitterConfig{
		ID:            id,
		Endpoint:      "http://localhost:8080/logs",
		Timeout:       10 * time.Second,
		RetryCount:    3,
		RetryDelay:    1 * time.Second,
		BatchSize:     3,
		FlushInterval: 1 * time.Second,
	}

	return &EmitterSimulator{
		ID:       id,
		Config:   config,
		Emitter:  emitters.NewHTTPEmitter(config),
		Packets:  packets,
		Interval: interval,
	}
}

// Run simulates the emitter sending packets
func (e *EmitterSimulator) Run(wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()

	log.Printf("Emitter %s starting - will send %d packets every %v", e.ID, e.Packets, e.Interval)

	for i := 0; i < e.Packets; i++ {
		// Create sample log messages
		messages := []models.LogMessage{
			{
				ID:        fmt.Sprintf("%s-msg-%d-1", e.ID, i),
				Timestamp: time.Now(),
				Level:     "INFO",
				Source:    fmt.Sprintf("app-%s", e.ID),
				Message:   fmt.Sprintf("Packet %d from %s", i+1, e.ID),
				Metadata: map[string]string{
					"emitter_id": e.ID,
					"packet_num": fmt.Sprintf("%d", i+1),
					"timestamp":  time.Now().Format(time.RFC3339),
				},
			},
			{
				ID:        fmt.Sprintf("%s-msg-%d-2", e.ID, i),
				Timestamp: time.Now().Add(time.Second),
				Level:     "DEBUG",
				Source:    fmt.Sprintf("app-%s", e.ID),
				Message:   fmt.Sprintf("Debug info for packet %d", i+1),
				Metadata: map[string]string{
					"emitter_id": e.ID,
					"debug":      "true",
				},
			},
		}

		// Create log packet
		packet := models.LogPacket{
			PacketID:  fmt.Sprintf("%s-packet-%d-%d", e.ID, time.Now().Unix(), i),
			AgentID:   e.ID,
			Timestamp: time.Now(),
			Messages:  messages,
		}

		// Send the packet
		start := time.Now()
		err := e.Emitter.Emit(packet)
		duration := time.Since(start)

		if err != nil {
			result := fmt.Sprintf("FAIL %s: Packet %d failed after %v - %v", e.ID, i+1, duration, err)
			results <- result
		} else {
			result := fmt.Sprintf("SUCCESS %s: Packet %d sent successfully in %v", e.ID, i+1, duration)
			results <- result
		}

		// Wait before sending next packet
		if i < e.Packets-1 { // Don't wait after the last packet
			time.Sleep(e.Interval)
		}
	}

	log.Printf("Emitter %s finished", e.ID)
}

func runMultiEmitterTest() {
	fmt.Println("=== Multi-Emitter Test ===")
	fmt.Println("This test simulates multiple emitters sending packets concurrently")
	fmt.Println("Make sure the distributor is running on http://localhost:8080")
	fmt.Println()

	// Configuration
	numEmitters := 10
	packetsPerEmitter := 3
	interval := 500 * time.Millisecond

	fmt.Printf("Starting %d emitters, each sending %d packets every %v\n", numEmitters, packetsPerEmitter, interval)
	fmt.Println()

	// Create emitters
	var wg sync.WaitGroup
	results := make(chan string, numEmitters*packetsPerEmitter)

	// Start all emitters concurrently
	startTime := time.Now()
	for i := 0; i < numEmitters; i++ {
		emitterID := fmt.Sprintf("emitter-%d", i+1)
		simulator := NewEmitterSimulator(emitterID, packetsPerEmitter, interval)

		wg.Add(1)
		go simulator.Run(&wg, results)
	}

	// Start a goroutine to collect and print results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Print results as they come in
	successCount := 0
	failureCount := 0

	for result := range results {
		fmt.Println(result)
		if strings.HasPrefix(result, "SUCCESS") {
			successCount++
		} else {
			failureCount++
		}
	}

	totalTime := time.Since(startTime)
	totalPackets := numEmitters * packetsPerEmitter

	fmt.Println()
	fmt.Println("=== Test Results ===")
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Total packets: %d\n", totalPackets)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failureCount)
	fmt.Printf("Success rate: %.1f%%\n", float64(successCount)/float64(totalPackets)*100)
	fmt.Printf("Average packets per second: %.1f\n", float64(totalPackets)/totalTime.Seconds())

	if failureCount == 0 {
		fmt.Println("🎉 All packets sent successfully!")
	} else {
		fmt.Printf("⚠️  %d packets failed\n", failureCount)
	}
}
