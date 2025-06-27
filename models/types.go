package main

import (
	"time"
)

// LogMessage represents a single log entry
type LogMessage struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`    // DEBUG, INFO, WARN, ERROR, FATAL
	Source    string            `json:"source"`   // application/service name
	Message   string            `json:"message"`  // actual log content
	Metadata  map[string]string `json:"metadata"` // additional context
}

// LogPacket represents a collection of log messages from an agent
type LogPacket struct {
	PacketID  string       `json:"packet_id"`
	AgentID   string       `json:"agent_id"`
	Timestamp time.Time    `json:"timestamp"`
	Messages  []LogMessage `json:"messages"`
}

// Analyzer interface for processing log messages
type Analyzer interface {
	Analyze(logMessage LogMessage) error
	GetID() string
	IsHealthy() bool
}

// AnalyzerConfig holds analyzer configuration
type AnalyzerConfig struct {
	ID         string  `json:"id"`
	Weight     float64 `json:"weight"`
	Enabled    bool    `json:"enabled"`
	Endpoint   string  `json:"endpoint"` // e.g., "http://analyzer1:8080"
	Timeout    int     `json:"timeout"`  // milliseconds
	RetryCount int     `json:"retry_count"`
}

// DistributorConfig holds the overall configuration
type DistributorConfig struct {
	Analyzers        []AnalyzerConfig
	Port             int
	TotalWeight      float64 // calculated field
	NormalizeWeights bool    // auto-normalize if needed
}

// Emitter interface for sending log packets to the distributor
type Emitter interface {
	Emit(packet LogPacket) error
	GetID() string
	GetEndpoint() string
}

// EmitterConfig holds emitter (agent) configuration
type EmitterConfig struct {
	ID             string
	Endpoint       string // distributor endpoint to send to
	Timeout        time.Duration
	RetryCount     int
	RetryDelay     time.Duration
	MaxConcurrency int
	BufferSize     int
	BatchSize      int           // max messages per packet
	FlushInterval  time.Duration // how often to send packets
}

// EmitterStatus tracks the health and performance of an emitter (agent)
type EmitterStatus struct {
	ID             string    `json:"id"`
	IsConnected    bool      `json:"is_connected"`
	LastConnection time.Time `json:"last_connection"`
	SuccessCount   int64     `json:"success_count"`
	FailureCount   int64     `json:"failure_count"`
	AverageLatency float64   `json:"average_latency"` // milliseconds
	LastError      string    `json:"last_error"`
	ActiveRequests int       `json:"active_requests"`
	MessagesSent   int64     `json:"messages_sent"`
	PacketsSent    int64     `json:"packets_sent"`
}

// EmitterMetrics holds performance metrics for an emitter (agent)
type EmitterMetrics struct {
	ID              string    `json:"id"`
	Timestamp       time.Time `json:"timestamp"`
	TotalPackets    int64     `json:"total_packets"`
	TotalMessages   int64     `json:"total_messages"`
	SuccessfulSends int64     `json:"successful_sends"`
	FailedSends     int64     `json:"failed_sends"`
	AverageLatency  float64   `json:"average_latency"`
	MinLatency      float64   `json:"min_latency"`
	MaxLatency      float64   `json:"max_latency"`
	Throughput      float64   `json:"throughput"` // packets per second
}

// EmitterPool manages multiple emitters (agents) that send to the distributor
type EmitterPool interface {
	GetEmitter(emitterID string) (Emitter, error)
	GetAllEmitters() map[string]Emitter
	UpdateEmitterStatus(emitterID string, status EmitterStatus)
	GetEmitterStatus(emitterID string) (EmitterStatus, error)
	GetPoolMetrics() map[string]EmitterMetrics
}
