#!/bin/bash

# Resolve Docker Management Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
}

# Function to build all services
build_all() {
    print_header "Building all services"
    check_docker
    docker-compose -f docker-compose.yml build
    print_status "All services built successfully"
}

# Function to start all services
start_all() {
    print_header "Starting all services"
    check_docker
    docker-compose -f docker-compose.yml up -d
    print_status "All services started successfully"
    print_status "Services are available at:"
    print_status "  Emitter Server: http://localhost:8080"
    print_status "  Distributor: http://localhost:8081"
    print_status "  Analyzer-1: http://localhost:8082"
    print_status "  Analyzer-2: http://localhost:8083"
    print_status "  Analyzer-3: http://localhost:8084"
}

# Function to stop all services
stop_all() {
    print_header "Stopping all services"
    docker-compose -f docker-compose.yml down
    print_status "All services stopped successfully"
}

# Function to restart all services
restart_all() {
    print_header "Restarting all services"
    stop_all
    start_all
}

# Function to show logs
show_logs() {
    local service=${1:-""}
    if [ -z "$service" ]; then
        print_header "Showing logs for all services"
        docker-compose -f docker-compose.yml logs -f
    else
        print_header "Showing logs for $service"
        docker-compose -f docker-compose.yml logs -f "$service"
    fi
}

# Function to show status
show_status() {
    print_header "Service Status"
    docker-compose -f docker-compose.yml ps
}

# Function to scale analyzers
scale_analyzers() {
    local count=${1:-3}
    print_header "Scaling analyzers to $count instances"
    
    # Stop existing analyzers
    docker-compose -f docker-compose.yml stop analyzer-1 analyzer-2 analyzer-3 2>/dev/null || true
    
    # Scale up
    docker-compose -f docker-compose.yml up -d --scale analyzer-1=1 --scale analyzer-2=1 --scale analyzer-3=1
    
    print_status "Scaled to $count analyzer instances"
}

# Function to clean up
cleanup() {
    print_header "Cleaning up Docker resources"
    docker-compose -f docker-compose.yml down --volumes --remove-orphans
    docker system prune -f
    print_status "Cleanup completed"
}

# Function to show health status
health_check() {
    print_header "Health Check"
    
    local services=("emitter-server:8080" "distributor:8081" "analyzer-1:8082" "analyzer-2:8083" "analyzer-3:8084")
    
    for service in "${services[@]}"; do
        local name=$(echo "$service" | cut -d: -f1)
        local port=$(echo "$service" | cut -d: -f2)
        
        if curl -s "http://localhost:$port/health" > /dev/null 2>&1; then
            print_status "$name: ${GREEN}HEALTHY${NC}"
        else
            print_error "$name: ${RED}UNHEALTHY${NC}"
        fi
    done
}

# Function to generate test logs
generate_logs() {
    print_header "Generating test logs"
    curl -X POST "http://localhost:8080/generate"
    print_status "Test logs generated"
}

# Function to start continuous log generation
start_log_generation() {
    print_header "Starting continuous log generation"
    curl -X POST "http://localhost:8080/start"
    print_status "Continuous log generation started"
}

# Function to verify weights
verify_weights() {
    echo "Fetching processed counts from analyzers..."
    
    # Get counts for each analyzer
    resp1=$(curl -s http://localhost:8082/processed)
    count1=$(echo "$resp1" | grep -o '"processed_count":[0-9]*' | grep -o '[0-9]*')
    
    resp2=$(curl -s http://localhost:8083/processed)
    count2=$(echo "$resp2" | grep -o '"processed_count":[0-9]*' | grep -o '[0-9]*')
    
    resp3=$(curl -s http://localhost:8084/processed)
    count3=$(echo "$resp3" | grep -o '"processed_count":[0-9]*' | grep -o '[0-9]*')
    
    total=$((count1 + count2 + count3))
    
    echo "analyzer-1 (weight 1.0): $count1 messages processed"
    echo "analyzer-2 (weight 2.0): $count2 messages processed"
    echo "analyzer-3 (weight 1.0): $count3 messages processed"
    echo
    
    if [ $total -gt 0 ]; then
        percent1=$(awk "BEGIN {printf \"%.2f\", ($count1/$total)*100}")
        percent2=$(awk "BEGIN {printf \"%.2f\", ($count2/$total)*100}")
        percent3=$(awk "BEGIN {printf \"%.2f\", ($count3/$total)*100}")
        
        echo "analyzer-1: $count1 messages ($percent1% of total, expected weight 1.0)"
        echo "analyzer-2: $count2 messages ($percent2% of total, expected weight 2.0)"
        echo "analyzer-3: $count3 messages ($percent3% of total, expected weight 1.0)"
        echo
        
        echo "Expected distribution (by weight):"
        echo "analyzer-1: 25.00%"
        echo "analyzer-2: 50.00%"
        echo "analyzer-3: 25.00%"
    else
        echo "No messages processed yet. Try generating some logs first."
    fi
}

# Function to show usage
show_usage() {
    echo "Resolve Docker Management Script"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  build           Build all services"
    echo "  start           Start all services"
    echo "  stop            Stop all services"
    echo "  restart         Restart all services"
    echo "  logs [SERVICE]  Show logs (all services or specific service)"
    echo "  status          Show service status"
    echo "  health          Check health of all services"
    echo "  scale N         Scale analyzers to N instances"
    echo "  cleanup         Clean up Docker resources"
    echo "  generate        Generate test logs"
    echo "  start-logs      Start continuous log generation"
    echo "  verify-weights  Verify analyzer processed counts"
    echo "  help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 logs emitter-server"
    echo "  $0 scale 5"
    echo "  $0 health"
}

# Main script logic
case "${1:-help}" in
    build)
        build_all
        ;;
    start)
        start_all
        ;;
    stop)
        stop_all
        ;;
    restart)
        restart_all
        ;;
    logs)
        show_logs "$2"
        ;;
    status)
        show_status
        ;;
    health)
        health_check
        ;;
    scale)
        scale_analyzers "$2"
        ;;
    cleanup)
        cleanup
        ;;
    generate)
        generate_logs
        ;;
    start-logs)
        start_log_generation
        ;;
    verify-weights)
        verify_weights
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        show_usage
        exit 1
        ;;
esac 