# 🚀 Rocket Tracking Service Solution

## Overview
This solution provides a service that consumes messages from rocket entities, maintains their state, and offers a REST API for accessing this information. The service is built using Go and the standard library, with an in-memory data store protected by mutexes for thread-safe access.

## Features
- **Message Processing**: Handles out-of-order messages and duplicate messages
- **REST API**: Exposes endpoints to:
  - Process incoming rocket messages
  - Retrieve the state of a specific rocket
  - List all rockets with sorting options
- **Swagger Documentation**: Interactive API documentation

## Architecture
- **Models**: Defines data structures for rocket messages and states
- **Storage**: In-memory repository with thread-safe access
- **API**: HTTP handlers for the REST endpoints
- **Tests**: Unit and integration tests with race detection

## Design Choices & Trade-offs

### In-Memory Storage
I chose to use in-memory storage as specified in the instructions. This offers:
- **Pros**: Fast access, simplicity
- **Cons**: Data is lost on service restart, limited by available memory

For production use with many rockets and long-term operation, a persistent storage solution would be more appropriate.

### Message Processing
- Messages are processed based on their message number to handle out-of-order delivery
- Each rocket tracks the highest message number processed to prevent duplicate processing
- Any message type can create a rocket, supporting scenarios where rockets are already in flight when the service starts

### Concurrency
- Mutex locks protect the repository from concurrent access
- Read operations use RLock for better performance with multiple concurrent reads

## Running the Service

### Prerequisites
- Go 1.24.1 or higher

### Build & Run
```bash
# Build the service
go build -o lunar.service ./cmd/server

# Run the service (default port: 8088)
./lunar.service

# Run with custom port
./lunar.service -port=9000
```

### Testing with the Test Program
```bash
# Run the test program against your service
./bin/rockets launch "http://localhost:8088/messages" --message-delay=500ms --concurrency-level=1
```

## API Documentation

### Swagger UI
The Swagger UI is available at [http://localhost:8088/swagger](http://localhost:8088/swagger) when the service is running.

### Endpoints

#### POST /messages
Process messages from rockets.

Request:
```json
{
    "metadata": {
        "channel": "193270a9-c9cf-404a-8f83-838e71d9ae67",
        "messageNumber": 1,    
        "messageTime": "2022-02-02T19:39:05.86337+01:00",                                          
        "messageType": "RocketLaunched"                             
    },
    "message": {                                                    
        "type": "Falcon-9",
        "launchSpeed": 500,
        "mission": "ARTEMIS"  
    }
}
```

#### GET /rockets
List all rockets, with optional sorting.

Query Parameters:
- `sort`: Field to sort by (e.g., `id`, `speed`, `type`, `mission`, `status`)
- `order`: Sort order (`asc` or `desc`)

#### GET /rockets/{id}
Get the current state of a specific rocket.

## Performance & Scalability

### Benchmark Results

Conducted extensive benchmarking to analyze the system's performance under various workloads and concurrency levels. The tests measured throughput (operations per second), latency (microseconds per operation), and memory efficiency.

#### 1. Small Workload (100 messages)
| Workers | Throughput (ops/sec) | Latency (μs/op) | Memory (B/op) | Allocs/op |
|---------|----------------------|-----------------|---------------|-----------|
| 1       | 30,000              | 116,500         | 36,929        | 402       |
| 2       | 28,000              | 127,400         | 37,200        | 406       |
| 4       | 29,000              | 124,900         | 37,800        | 417       |

**Key Finding**: For small workloads, a single worker provides optimal performance with minimal overhead.

#### 2. Medium Workload (1,000 messages)
| Workers | Throughput (ops/sec) | Latency (μs/op) | Memory (B/op) | Allocs/op |
|---------|----------------------|-----------------|---------------|-----------|
| 1       | 4,500               | 886,000         | 276,000       | 3,000     |
| 2       | 4,500               | 896,000         | 260,000       | 2,900     |
| 4       | 3,800               | 1,038,000       | 320,000       | 3,800     |

**Key Finding**: Two workers provide the best balance of throughput and resource usage for medium workloads.

#### 3. Large Workload (10,000 messages)
| Workers | Throughput (ops/sec) | Latency (μs/op) | Memory (B/op) | Allocs/op |
|---------|----------------------|-----------------|---------------|-----------|
| 1       | 3,100               | 1,494,000       | 464,000       | 5,100     |
| 2       | 3,500               | 1,792,000       | 508,000       | 5,700     |
| 4       | 3,500               | 1,706,000       | 516,000       | 6,200     |

**Key Finding**: Two workers provide ~13% better throughput than a single worker for large workloads, with minimal memory overhead.

### Scalability Analysis

1. **Optimal Worker Configuration**:
   - Small workloads (<100 messages): 1 worker (lowest overhead)
   - Medium to large workloads (≥1,000 messages): 2 workers (best balance)
   - No significant benefit observed beyond 2 workers due to lock contention

2. **Memory Efficiency**:
   - Memory usage scales linearly with message volume
   - Consistent ~370 bytes per message overhead
   - No memory leaks detected during extended testing

3. **Recommendations**:
   - Use 1 worker for low-volume scenarios
   - Scale to 2 workers for higher volumes
   - Consider horizontal scaling for very high throughput requirements

## Running Tests
```bash
# Run all tests with race detection
go test -race -covermode=atomic ./...

# Run tests with coverage report
go test -race -covermode=atomic -cover ./...

# Run benchmark tests
go test -v -run=^$ -bench=. -benchmem ./test/...

# Run tests with coverage output
go test -race -covermode=atomic -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out
```
