# LocalCloud Component Test Suite

A comprehensive, modular testing framework for LocalCloud components. This test suite validates each component individually by setting up the service, testing functionality, and cleaning up resources.

## Overview

The test suite provides:
- **Modular testing**: Each component tested independently with proper isolation
- **Event-driven readiness**: Smart service monitoring instead of fixed timeouts  
- **Cross-platform support**: Works on macOS, Linux with native timeout handling
- **Robust interruption**: Proper Ctrl+C handling and process cleanup
- **GitHub Actions compatible**: Structured output for CI/CD integration
- **Real-world testing**: Uses actual `lc` commands as users would
- **Comprehensive coverage**: Tests all major LocalCloud components
- **Smart dependency detection**: Understands component relationships (e.g., database ↔ vector)

## Components Tested

| Component | Description | Test Coverage |
|-----------|-------------|---------------|
| **database** | PostgreSQL relational database | Connection, tables, CRUD operations, performance |
| **vector** | pgvector extension for PostgreSQL | Extension setup, vector operations, similarity search, indexing |
| **mongodb** | MongoDB document database | Connection, databases, collections, documents, indexing |
| **cache** | Redis cache service | Key-value operations, expiration, data types, performance |
| **queue** | Redis job queue service | Job queuing, processing patterns, persistence, performance |
| **storage** | MinIO object storage | Bucket operations, file upload/download, large files |
| **embedding** | AI text embeddings | Model availability, embedding generation, similarity |
| **llm** | Large language models | Model loading, text generation, streaming, performance |

## Quick Start

### Prerequisites

1. **LocalCloud CLI**: Install using one of the methods below
2. **Docker**: Required for running services
3. **Required tools**: `curl`, `jq` (optional but recommended)
4. **Database clients** (optional): `psql`, `mongosh`, `redis-cli` for enhanced testing

### Platform Compatibility

The test suite works across different platforms with automatic adaptation:

- **macOS**: Uses native bash timeout implementation when `timeout` is not available
- **Linux**: Uses GNU `timeout` command when available  
- **Windows (WSL)**: Supported through WSL with Linux compatibility
- **GitHub Actions**: Optimized for CI/CD environments with structured output

**Timeout Handling:**
- Automatic detection of available timeout commands (`timeout`, `gtimeout`)
- Falls back to bash-native timeout implementation on macOS
- 60-second timeout protection for component operations
- Proper process cleanup on interruption

#### Installing LocalCloud CLI

**macOS (Recommended):**
```bash
# Using Homebrew
brew install localcloud-sh/tap/localcloud
```

**Linux:**
```bash
# Using Homebrew (if available)
brew install localcloud-sh/tap/localcloud

# Or using install script
curl -fsSL https://raw.githubusercontent.com/localcloud-sh/localcloud/main/scripts/install.sh | bash
```

**Manual Installation:**
See the [main LocalCloud README](../README.md) for manual installation instructions for your platform.

### Basic Usage

```bash
# Run all component tests
./test-runner.sh

# Test specific components
./test-runner.sh --components database,vector,cache

# Parallel testing
./test-runner.sh --parallel 2

# GitHub Actions format
./test-runner.sh --format json --output ./reports

# Verbose output
./test-runner.sh --verbose
```

## Command Line Options

```bash
./test-runner.sh [OPTIONS]

Options:
  -c, --components LIST    Comma-separated list of components to test
                          Default: all components
  -p, --parallel N         Run N tests in parallel (default: 1)
  -t, --timeout N          Timeout in seconds for each test (default: 600)
  -f, --format FORMAT      Output format: console, json, junit (default: console)
  -o, --output DIR         Output directory for reports (default: ./reports)
  --no-cleanup            Don't cleanup on errors (for debugging)
  -v, --verbose           Verbose output
  -h, --help              Show help
```

## Examples

### Test All Components
```bash
./test-runner.sh
```

### Test Database Components Only
```bash
./test-runner.sh --components database,vector,mongodb
```

### Parallel Testing with JSON Output
```bash
./test-runner.sh --parallel 2 --format json --output ./ci-reports
```

### Test LLM with Extended Timeout
```bash
./test-runner.sh --components llm --timeout 900 --verbose
```

### Debug Mode (No Cleanup)
```bash
./test-runner.sh --components cache --no-cleanup --verbose
```

## GitHub Actions Integration

### Basic Workflow

```yaml
name: LocalCloud Component Tests

on: [push, pull_request]

jobs:
  test-components:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Setup LocalCloud
      run: |
        # Install LocalCloud CLI
        curl -fsSL https://raw.githubusercontent.com/localcloud-sh/localcloud/main/scripts/install.sh | bash
        
    - name: Run Component Tests
      run: |
        cd test-components
        chmod +x test-runner.sh
        ./test-runner.sh --format junit --output ./reports
        
    - name: Upload Test Results
      uses: actions/upload-artifact@v4
      if: always()
      with:
        name: test-results
        path: test-components/reports/
```

### Matrix Testing

```yaml
strategy:
  matrix:
    component-group:
      - "database,vector"
      - "mongodb,cache,queue"
      - "storage"
      - "llm"
      - "embedding"

steps:
- name: Test Component Group
  run: |
    ./test-runner.sh \
      --components ${{ matrix.component-group }} \
      --format json \
      --output ./reports/${{ matrix.component-group }}
```

## Architecture

### Directory Structure

```
test-components/
├── test-runner.sh          # Main test orchestrator
├── lib/                    # Common libraries
│   ├── common.sh          # Utility functions
│   ├── health-monitor.sh  # Service readiness monitoring
│   └── reporter.sh        # Test result reporting
├── components/            # Component-specific tests
│   ├── test-database.sh   # PostgreSQL tests
│   ├── test-vector.sh     # pgvector tests
│   ├── test-mongodb.sh    # MongoDB tests
│   ├── test-cache.sh      # Redis cache tests
│   ├── test-queue.sh      # Redis queue tests
│   ├── test-storage.sh    # MinIO tests
│   ├── test-embedding.sh  # Embedding tests
│   └── test-llm.sh        # LLM tests
├── reports/               # Test output directory
└── README.md             # This file
```

### Test Flow

Each component test follows this pattern:

1. **Setup**: Add component to LocalCloud project
2. **Start**: Start the service using `lc start`
3. **Wait**: Wait for service to be healthy (event-driven)
4. **Test**: Run comprehensive functionality tests
5. **Cleanup**: Remove component and stop services

### Event-Driven Service Monitoring

Instead of fixed timeouts, the framework uses:
- **Health checks**: Service-specific health validation
- **Port monitoring**: TCP connectivity checks
- **API validation**: Endpoint-specific readiness tests
- **Container monitoring**: Docker container health status

## Component Test Details

### Database Component (`test-database.sh`)

Tests PostgreSQL functionality:
- Connection establishment
- Table creation and management
- CRUD operations (Create, Read, Update, Delete)
- Performance benchmarking
- Transaction handling

### Vector Component (`test-vector.sh`)

Tests pgvector extension:
- Extension installation and availability
- Vector table creation
- Vector data insertion and querying
- Similarity search (cosine, L2 distance)
- Vector indexing (HNSW, IVFFlat)

### MongoDB Component (`test-mongodb.sh`)

Tests MongoDB functionality:
- Connection and authentication
- Database and collection operations
- Document CRUD operations
- Indexing (single field, compound, text)
- Aggregation pipelines

### Cache Component (`test-cache.sh`)

Tests Redis cache:
- Basic key-value operations
- Key expiration and TTL
- Redis data types (strings, hashes, lists, sets)
- Performance testing
- Memory management

### Queue Component (`test-queue.sh`)

Tests Redis job queue:
- Job queuing and processing
- FIFO and blocking operations
- Reliable queue patterns (RPOPLPUSH)
- Persistence validation
- Batch processing performance

### Storage Component (`test-storage.sh`)

Tests MinIO object storage:
- Bucket operations
- Object upload and download
- Large file handling
- Performance testing
- S3 API compatibility

### Embedding Component (`test-embedding.sh`)

Tests AI text embeddings:
- Model availability and loading
- Text-to-vector conversion
- Embedding similarity testing
- Batch processing
- Vector dimension validation

### LLM Component (`test-llm.sh`)

Tests large language models:
- Model availability and loading
- Text generation
- Streaming responses
- Performance benchmarking
- API endpoint validation

## Output Formats

### Console Output (Default)

```
[2024-01-15 10:30:00] ✓ database: PASS (45s)
[2024-01-15 10:31:15] ✗ llm: FAIL (180s) - Model loading timeout
```

### JSON Format

```json
{
    "timestamp": "2024-01-15T10:30:00Z",
    "tests": [
        {
            "component": "database",
            "result": "PASS",
            "duration": 45,
            "error": ""
        }
    ],
    "summary": {
        "total": 8,
        "passed": 7,
        "failed": 1,
        "duration": 420
    }
}
```

### JUnit XML Format

```xml
<testsuites>
    <testsuite name="LocalCloud Component Tests" tests="8" failures="1" time="420">
        <testcase name="database" classname="LocalCloud.Component" time="45" />
        <testcase name="llm" classname="LocalCloud.Component" time="180">
            <failure message="Model loading timeout">Model loading timeout</failure>
        </testcase>
    </testsuite>
</testsuites>
```

## Troubleshooting

### Common Issues

1. **Tests hanging during component addition**
   ```bash
   # The test suite now includes timeout protection and Ctrl+C handling
   # If a test hangs, you can safely interrupt with Ctrl+C
   # The system will automatically clean up hanging processes
   ```

2. **Docker not running**
   ```bash
   # Solution: Start Docker
   sudo systemctl start docker
   ```

3. **LocalCloud CLI not found**
   ```bash
   # Solution: Install LocalCloud CLI
   brew install localcloud-sh/tap/localcloud
   # Or use the install script
   curl -fsSL https://raw.githubusercontent.com/localcloud-sh/localcloud/main/scripts/install.sh | bash
   ```

4. **Port conflicts**
   ```bash
   # Solution: Stop conflicting services
   ./test-runner.sh --components database --no-cleanup --verbose
   ```

5. **Model loading timeout (LLM/Embedding)**
   ```bash
   # Solution: Increase timeout and use verbose mode for progress
   ./test-runner.sh --components llm --timeout 1200 --verbose
   ```

6. **"Test isolation issue" warnings**
   ```bash
   # The system now understands component dependencies:
   # - database + vector is expected (vector extends PostgreSQL)
   # - Other combinations will still show warnings
   ```

### Debug Mode

Use `--no-cleanup` and `--verbose` for debugging:

```bash
./test-runner.sh --components database --no-cleanup --verbose
```

This will:
- Keep services running after test completion
- Show detailed debug output
- Allow manual inspection of service state

### Monitoring Long-Running Tests

For AI components (LLM, embedding) that may take time to download models:

1. **Use verbose mode** to see progress updates:
   ```bash
   ./test-runner.sh --components llm --verbose
   ```

2. **Monitor in real-time** with the progress monitor:
   ```bash
   # In a separate terminal
   ./monitor-llm-progress.sh
   ```

3. **Progress indicators**:
   - Updates every 30 seconds during AI service startup
   - Container activity logs in verbose mode
   - Clear timeout information

### Log Files

Test logs are saved in the reports directory:
- `console.log`: Human-readable test log
- `test-results.json`: Machine-readable results
- `junit.xml`: CI/CD compatible results

## Performance Considerations

### Resource Requirements

- **Minimum RAM**: 4GB (for basic components)
- **Recommended RAM**: 8GB (for AI components)
- **CPU**: 2+ cores recommended for parallel testing
- **Disk Space**: 10GB+ for all components

### GitHub Actions Optimization

- Use component groups to avoid resource exhaustion
- Cache Docker images between runs
- Set appropriate timeouts for AI components
- Monitor resource usage during tests

### Timeout Guidelines

| Component | Recommended Timeout |
|-----------|-------------------|
| database, cache, queue | 120s |
| mongodb, storage | 180s |
| vector | 240s |
| embedding | 600s |
| llm | 900s |

## Contributing

### Adding New Component Tests

1. Create new test script in `components/`:
   ```bash
   cp components/test-database.sh components/test-newservice.sh
   ```

2. Update the script for your component
3. Add component to `AVAILABLE_COMPONENTS` in `test-runner.sh`
4. Test thoroughly with different configurations

### Improving Existing Tests

1. Focus on real-world usage patterns
2. Add performance benchmarks
3. Improve error handling and diagnostics
4. Enhance GitHub Actions compatibility

## License

This test suite is part of the LocalCloud project and follows the same license terms.