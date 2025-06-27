package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"resolve/models"
)

// DistributorServer handles incoming log packets from emitters
type DistributorServer struct {
	port int
}

// NewDistributorServer creates a new distributor server
func NewDistributorServer(port int) *DistributorServer {
	return &DistributorServer{
		port: port,
	}
}

// Start starts the HTTP server
func (d *DistributorServer) Start() error {
	// Set up routes
	http.HandleFunc("/logs", d.handleLogPacket)
	http.HandleFunc("/health", d.handleHealth)

	// Start server
	addr := fmt.Sprintf(":%d", d.port)
	log.Printf("Distributor server starting on port %d", d.port)
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

	// Print the received packet
	d.printLogPacket(packet)

	// Send success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Log packet received successfully"))
}

// handleHealth provides a health check endpoint
func (d *DistributorServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Distributor is healthy"))
}

// printLogPacket prints the received log packet in a readable format
func (d *DistributorServer) printLogPacket(packet models.LogPacket) {
	fmt.Printf("\n=== LOG PACKET RECEIVED ===\n")
	fmt.Printf("Packet ID: %s\n", packet.PacketID)
	fmt.Printf("Agent ID: %s\n", packet.AgentID)
	fmt.Printf("Timestamp: %s\n", packet.Timestamp.Format(time.RFC3339))
	fmt.Printf("Number of messages: %d\n", len(packet.Messages))
	fmt.Printf("Messages:\n")

	for i, msg := range packet.Messages {
		fmt.Printf("  [%d] %s [%s] %s: %s\n",
			i+1,
			msg.Timestamp.Format("15:04:05"),
			msg.Level,
			msg.Source,
			msg.Message)

		// Print metadata if present
		if len(msg.Metadata) > 0 {
			fmt.Printf("      Metadata: %v\n", msg.Metadata)
		}
	}
	fmt.Printf("=== END PACKET ===\n\n")
}

func main() {
	// Create and start the distributor server
	server := NewDistributorServer(8080)

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start distributor server: %v", err)
	}
}
