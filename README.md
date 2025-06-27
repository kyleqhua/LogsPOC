# Distributed Log Processing System

A POC distributed log processing system with emitter servers, distributors, and analyzers, all containerized with Docker. Features basic automatic message queuing, rerouting, and zero message loss even when analyzers fail.

## Write up:
[Project Write Up and Thoughts](https://docs.google.com/document/d/1AUoJjDDUgHR_ffd0-jCmaHaVdjNPOoz8drDgOIonCEI/edit?tab=t.0)

## Features

- **Zero Message Loss**: Queue system ensures no messages are lost even when analyzers are down
- **Automatic Rerouting**: Failed messages are automatically rerouted to healthy analyzers
- **Dynamic Analyzer Control**: Enable/disable analyzers on-the-fly to simulate failures
- **Weighted Load Balancing**: Distribution based on analyzer weights, re weights made if analyzer goes down
- **Monitoring**: Real-time health checks and queue status monitoring + message counting and distribution 

## Architecture

The system consists of three main components:

1. **Emitter Server** - Generates log messages and sends them to distributors (250 messages per generate call or 10 logs / sec)
2. **Distributor** - Receives logs, distributes them using weighted load balancing, and queues failed messages for retry
3. **Analyzers** - Process individual log messages with enable/disable capability (multiple instances supported)

## Docker Setup

### Prerequisites

- Docker
- Docker Compose

### Quick Start

1. **Build and start all services (from root directory):**
   ```bash
   ./docker.sh start
   ```

2. **Or from the docker directory:**
   ```bash
   cd docker
   ./docker-scripts.sh start
   ```

3. **Or use docker-compose directly:**
   ```bash
   cd docker
   docker-compose up -d --build
   ```

4. **Generate test logs:**
   ```bash
   cd docker
   ./docker-scripts.sh generate
   ```

5. **Verify message distribution:**
   ```bash
   cd docker
   ./docker-scripts.sh verify-weights
   ```

6. **View logs:**
   ```bash
   # From root directory
   ./docker.sh logs
   
   # From docker directory
   cd docker
   ./docker-scripts.sh logs
   
   # Specific service
   ./docker.sh logs emitter-server
   ./docker.sh logs distributor
   ./docker.sh logs analyzer-1
   ```

7. **Stop all services:**
   ```bash
   # From root directory
   ./docker.sh stop
   
   # From docker directory
   cd docker
   ./docker-scripts.sh stop
   ```

### Service Endpoints

Once running, the following endpoints are available:

#### Emitter Server (Port 8080)
- `GET /health` - Health check
- `GET /stats` - Statistics and configuration
- `POST /start` - Start continuous log generation
- `POST /stop` - Stop log generation
- `POST /generate` - Generate a single batch of logs (250 total messages)

#### Distributor (Port 8081)
- `GET /health` - Health check
- `GET /queue` - Queue status (size, oldest message age)
- `POST /logs` - Receive log packets from emitters

#### Analyzers (Ports 8082, 8083, 8084)
- `GET /health` - Health check
- `GET /status` - Detailed status information (enabled/disabled, healthy/unhealthy)
- `GET /processed` - Number of messages processed
- `POST /analyze` - Analyze a log message
- `POST /enable` - Enable the analyzer
- `POST /disable` - Disable the analyzer

### Testing Fault Tolerance

#### Simulate Analyzer Failure
```bash
# Disable an analyzer, change port per
curl -X POST http://localhost:8082/disable 

# Generate logs (messages will be queued/rerouted)
./docker-scripts.sh generate

# Check queue status
curl -s http://localhost:8081/queue

# Verify distribution (disabled analyzer should have 0 new messages)
./docker-scripts.sh verify-weights
```

#### Test Recovery
```bash
# Re-enable the analyzer
curl -X POST http://localhost:8082/enable

# Generate more logs
./docker-scripts.sh generate

# Verify all analyzers are processing again
./docker-scripts.sh verify-weights
```

### Configuration

#### Emitter Server Configuration
Edit `emitterServer/docker_config.json` to configure the emitter server:

```json
{
  "port": 8080,
  "distributor_urls": ["http://distributor:8080/logs"],
  "log_generation_rate": 10,
  "max_concurrency": 5,
  "batch_size": 50,
  "flush_interval": 1000,
  "emitters_per_distributor": 5
}
```

**Configuration Options:**
- `port`: HTTP server port
- `distributor_urls`: Array of distributor endpoints
- `log_generation_rate`: Logs generated per second
- `max_concurrency`: Maximum concurrent operations
- `batch_size`: Number of messages per packet (50 × 5 emitters = 250 total messages per generate)
- `flush_interval`: Flush interval in milliseconds
- `emitters_per_distributor`: Number of emitters created per distributor (default: 5)

#### Distributor Configuration
Edit `distributor/docker_config.json` to configure analyzers:

```json
{
  "port": 8080,
  "analyzers": [
    {
      "id": "analyzer-1",
      "weight": 1.0,
      "endpoint": "http://analyzer-1:8081/analyze",
      "timeout": 10000,
      "retry_count": 3
    }
  ]
}
```

**Configuration Options:**
- `port`: HTTP server port
- `analyzers`: Array of analyzer configurations
  - `id`: Unique analyzer identifier
  - `weight`: Load balancing weight (higher = more messages)
  - `endpoint`: Analyzer's analyze endpoint
  - `timeout`: Request timeout in milliseconds
  - `retry_count`: Number of retry attempts before queuing

#### Analyzer Configuration
Analyzers are configured via command-line arguments:
- First argument: Analyzer ID
- Second argument: Port number

### Message Flow and Reliability

#### Normal Operation
1. **Emitter** generates 250 messages (50 × 5 emitters) per `generate` call
2. **Distributor** receives messages and distributes them using weighted load balancing
3. **Analyzers** process messages and increment their processed count

#### Fault Tolerance
1. **Failed Delivery**: If an analyzer is disabled or unreachable, the distributor retries up to `retry_count` times
2. **Message Queuing**: After all retries fail, messages are queued for later delivery
3. **Automatic Rerouting**: Background worker attempts to deliver queued messages to alternative analyzers
4. **Zero Loss**: Messages remain in queue until successfully delivered to any available analyzer

#### Queue Management
- **Background Processing**: Queue is processed every 2 seconds
- **Alternative Selection**: Queued messages are sent to analyzers not previously tried
- **Weighted Distribution**: Even queued messages follow weighted distribution among available analyzers
- **Monitoring**: Queue status available via `/queue` endpoint

### Scaling

#### Add More Analyzers

1. **Add new analyzer service to `docker/docker-compose.yml`:**
   ```yaml
   analyzer-4:
     build:
       context: ..
       dockerfile: docker/Dockerfile.analyzer
     ports:
       - "8085:8084"
     command: ["./analyzer", "analyzer-4", "8084"]
     networks:
       - resolve-network
     restart: unless-stopped
   ```

2. **Update distributor configuration in `distributor/docker_config.json`:**
   ```json
   {
     "id": "analyzer-4",
     "weight": 1.5,
     "endpoint": "http://analyzer-4:8084/analyze",
     "timeout": 10000,
     "retry_count": 3
   }
   ```

3. **Restart the distributor:**
   ```bash
   ./docker.sh restart distributor
   ```

#### Add More Distributors

1. **Add new distributor service to `docker/docker-compose.yml`:**
   ```yaml
   distributor-2:
     build:
       context: ..
       dockerfile: docker/Dockerfile.distributor
     ports:
       - "8086:8080"
     volumes:
       - ../distributor/docker_config.json:/root/config.json
     depends_on:
       - analyzer-1
       - analyzer-2
       - analyzer-3
     networks:
       - resolve-network
     restart: unless-stopped
   ```

2. **Update emitter configuration to include the new distributor:**
   ```json
   {
     "distributor_urls": [
       "http://distributor:8080/logs",
       "http://distributor-2:8080/logs"
     ]
   }
   ```

### Monitoring

#### Health Checks
All services provide health check endpoints:
- Emitter: `http://localhost:8080/health`
- Distributor: `http://localhost:8081/health`
- Analyzers: `http://localhost:8082/health`, `http://localhost:8083/health`, `http://localhost:8084/health`

#### Statistics and Status
- Emitter stats: `http://localhost:8080/stats`
- Distributor queue status: `http://localhost:8081/queue`
- Analyzer processed counts: `http://localhost:8082/processed`, etc.
- Analyzer status: `http://localhost:8082/status`, etc.

#### Message Distribution Verification
```bash
cd docker
./docker-scripts.sh verify-weights
```
Shows:
- Messages processed by each analyzer
- Distribution percentages vs expected weights
- Total messages processed vs expected (250 per generate call)

### Troubleshooting

#### Check Service Status
```bash
./docker.sh status
```

#### View Service Logs
```bash
./docker.sh logs [service-name]
```

#### Check Queue Status
```bash
curl -s http://localhost:8081/queue
```

#### Restart a Service
```bash
cd docker
docker-compose restart [service-name]
```

#### Rebuild a Service
```bash
cd docker
docker-compose up --build [service-name]
```


## Network Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Emitter       │    │   Distributor   │    │   Analyzer-1    │
│   Server        │───▶│   (with Queue)  │───▶│   (Enable/      │
│   (Port 8080)   │    │   (Port 8081)   │    │    Disable)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Analyzer-2    │    │   Analyzer-3    │
                       │   (Enable/      │    │   (Enable/      │
                       │    Disable)     │    │    Disable)     │
                       └─────────────────┘    └─────────────────┘
```

## Performance Tuning

### Emitter Server
- Increase `log_generation_rate` for higher throughput
- Adjust `batch_size` for optimal packet size
- Tune `max_concurrency` based on available resources

### Distributor
- Modify worker pool size in the distributor code
- Adjust analyzer weights for load balancing
- Configure timeouts and retry counts per analyzer
- Queue processing interval (currently 2 seconds)

### Analyzers
- Each analyzer runs in its own container
- Scale horizontally by adding more analyzer instances
- Monitor processing times and adjust accordingly
- Use enable/disable for maintenance or load testing