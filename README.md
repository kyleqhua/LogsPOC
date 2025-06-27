# Resolve - Distributed Log Processing System

A distributed log processing system with emitter servers, distributors, and analyzers, all containerized with Docker.

## Architecture

The system consists of three main components:

1. **Emitter Server** - Generates log messages and sends them to distributors
2. **Distributor** - Receives logs and distributes them to analyzers using weighted load balancing
3. **Analyzers** - Process individual log messages (multiple instances supported)

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

4. **View logs:**
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

5. **Stop all services:**
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
- `POST /generate` - Generate a single batch of logs

#### Distributor (Port 8081)
- `GET /health` - Health check
- `POST /logs` - Receive log packets from emitters

#### Analyzers (Ports 8082, 8083, 8084)
- `GET /health` - Health check
- `GET /status` - Detailed status information
- `GET /processed` - Number of messages processed
- `POST /analyze` - Analyze a log message

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
- `batch_size`: Number of messages per packet
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
      "enabled": true,
      "endpoint": "http://analyzer-1:8081/analyze",
      "timeout": 10000,
      "retry_count": 3
    }
  ]
}
```

#### Analyzer Configuration
Analyzers are configured via command-line arguments:
- First argument: Analyzer ID
- Second argument: Port number

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
     "enabled": true,
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

#### Statistics
- Emitter stats: `http://localhost:8080/stats`
- Analyzer processed counts: `http://localhost:8082/processed`, etc.

### Troubleshooting

#### Check Service Status
```bash
./docker.sh status
```

#### View Service Logs
```bash
./docker.sh logs [service-name]
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

### Development

#### Building Individual Services
```bash
cd docker
# Build emitter server
docker build -f Dockerfile.emitter -t resolve-emitter ..

# Build distributor
docker build -f Dockerfile.distributor -t resolve-distributor ..

# Build analyzer
docker build -f Dockerfile.analyzer -t resolve-analyzer ..
```

#### Running Individual Containers
```bash
# Run emitter server
docker run -p 8080:8080 resolve-emitter

# Run distributor
docker run -p 8081:8080 resolve-distributor

# Run analyzer
docker run -p 8082:8081 resolve-analyzer ./analyzer analyzer-1 8081
```

## Project Structure

```
resolve/
├── docker/                    # Docker configuration files
│   ├── Dockerfile.emitter     # Emitter server Dockerfile
│   ├── Dockerfile.distributor # Distributor Dockerfile
│   ├── Dockerfile.analyzer    # Analyzer Dockerfile
│   ├── docker-compose.yml     # Docker Compose configuration
│   ├── .dockerignore          # Docker ignore file
│   └── docker-scripts.sh      # Management script
├── docker.sh                  # Wrapper script (run from root)
├── emitterServer/             # Emitter server source code
│   ├── emitter_server.go      # Main emitter server code
│   ├── config.json            # Local development config
│   └── docker_config.json     # Docker deployment config
├── distributor/               # Distributor source code
│   ├── distributor.go         # Main distributor code
│   ├── local_config.json      # Local development config
│   └── docker_config.json     # Docker deployment config
├── analyzers/                 # Analyzer source code
├── emitters/                  # Emitter implementations
├── models/                    # Data models
└── README.md                  # This file
```

## Network Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Emitter       │    │   Distributor   │    │   Analyzer-1    │
│   Server        │───▶│                 │───▶│                 │
│   (Port 8080)   │    │   (Port 8081)   │    │   (Port 8082)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Analyzer-2    │    │   Analyzer-3    │
                       │   (Port 8083)   │    │   (Port 8084)   │
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

### Analyzers
- Each analyzer runs in its own container
- Scale horizontally by adding more analyzer instances
- Monitor processing times and adjust accordingly 