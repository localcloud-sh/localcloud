#!/bin/bash

# LocalCloud Component Test Runner
# Tests each component individually: setup -> start -> test -> cleanup

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"
COMPONENTS_DIR="$SCRIPT_DIR/components"
REPORTS_DIR="$SCRIPT_DIR/reports"

# Source common functions
source "$LIB_DIR/common.sh"
source "$LIB_DIR/health-monitor.sh"
source "$LIB_DIR/reporter.sh"

# Default values
COMPONENTS_TO_TEST="database,vector,mongodb,cache,queue,storage,embedding,llm"
PARALLEL_JOBS=1
TIMEOUT=600
OUTPUT_FORMAT="console"
CLEANUP_ON_ERROR=true
VERBOSE=false

# Test results (bash 4+ associative arrays)
if [[ ${BASH_VERSION%%.*} -ge 4 ]]; then
    declare -A TEST_RESULTS=()
    declare -A TEST_DURATIONS=()
else
    # Fallback for older bash versions - use simple arrays
    TEST_RESULT_KEYS=()
    TEST_RESULT_VALUES=()
    TEST_DURATION_KEYS=()
    TEST_DURATION_VALUES=()
fi

# Available components
AVAILABLE_COMPONENTS=(
    "database"      # PostgreSQL
    "vector"        # pgvector
    "mongodb"       # MongoDB
    "cache"         # Redis cache
    "queue"         # Redis queue
    "storage"       # MinIO
    "embedding"     # AI embeddings
    "llm"           # Large language models
)

show_help() {
    cat << EOF
LocalCloud Component Test Runner

Usage: $0 [OPTIONS]

Options:
    -c, --components LIST    Comma-separated list of components to test
                            Default: all components
    -p, --parallel N         Run N tests in parallel (default: 1)
    -t, --timeout N          Timeout in seconds for each test (default: 600)
    -f, --format FORMAT      Output format: console, json, junit (default: console)
    -o, --output DIR         Output directory for reports (default: ./reports)
    --no-cleanup            Don't cleanup on errors (for debugging)
    -v, --verbose           Verbose output
    -h, --help              Show this help

Components:
    database    - PostgreSQL database
    vector      - pgvector extension
    mongodb     - MongoDB document database
    cache       - Redis cache
    queue       - Redis job queue
    storage     - MinIO object storage
    embedding   - AI text embeddings
    llm         - Large language models

Examples:
    $0                                    # Test all components
    $0 -c database,vector                 # Test only database and vector
    $0 -p 2 -f json                      # Parallel testing with JSON output
    $0 -c llm -t 900 -v                  # Test LLM with 15min timeout, verbose
    $0 --format junit -o ./ci-reports    # Generate JUnit XML for CI

EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -c|--components)
                COMPONENTS_TO_TEST="$2"
                shift 2
                ;;
            -p|--parallel)
                PARALLEL_JOBS="$2"
                shift 2
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -f|--format)
                OUTPUT_FORMAT="$2"
                shift 2
                ;;
            -o|--output)
                REPORTS_DIR="$2"
                shift 2
                ;;
            --no-cleanup)
                CLEANUP_ON_ERROR=false
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

validate_args() {
    # Validate components
    IFS=',' read -ra COMPONENTS <<< "$COMPONENTS_TO_TEST"
    for component in "${COMPONENTS[@]}"; do
        if [[ ! " ${AVAILABLE_COMPONENTS[*]} " =~ " ${component} " ]]; then
            log_error "Invalid component: $component"
            log_info "Available components: ${AVAILABLE_COMPONENTS[*]}"
            exit 1
        fi
    done
    
    # Validate parallel jobs
    if [[ ! "$PARALLEL_JOBS" =~ ^[0-9]+$ ]] || [[ "$PARALLEL_JOBS" -lt 1 ]]; then
        log_error "Invalid parallel jobs: $PARALLEL_JOBS"
        exit 1
    fi
    
    # Validate timeout
    if [[ ! "$TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT" -lt 60 ]]; then
        log_error "Invalid timeout: $TIMEOUT (minimum 60 seconds)"
        exit 1
    fi
    
    # Validate output format
    if [[ ! "$OUTPUT_FORMAT" =~ ^(console|json|junit)$ ]]; then
        log_error "Invalid output format: $OUTPUT_FORMAT"
        exit 1
    fi
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if LocalCloud is available
    if ! command -v lc &> /dev/null; then
        log_error "LocalCloud CLI (lc) not found in PATH"
        log_info "Please install LocalCloud CLI:"
        log_info "  macOS: brew install localcloud-sh/tap/localcloud"
        log_info "  Linux: curl -fsSL https://raw.githubusercontent.com/localcloud-sh/localcloud/main/scripts/install.sh | bash"
        exit 1
    fi
    
    # Check required tools
    local required_tools=("curl" "docker")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log_error "Required tool not found: $tool"
            exit 1
        fi
    done
    
    # Check if jq is available (optional but recommended)
    if ! command -v jq &> /dev/null; then
        log_warning "jq not found - some advanced JSON parsing features will be limited"
    fi
    
    # Check Docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker is not running"
        log_info "Please start Docker and try again"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

setup_test_project() {
    log_info "Setting up LocalCloud test project..."
    
    # Always start with a fresh project for proper test isolation
    if [[ -f ".localcloud/config.yaml" ]]; then
        log_info "Removing existing project for clean test isolation"
        rm -rf .localcloud
    fi
    
    # Create a temporary test project
    log_info "Creating temporary LocalCloud test project..."
    
    # Try to create a minimal project configuration
    if ! lc setup test-components-project --non-interactive &>/dev/null; then
        # Fallback: try interactive setup with default selections
        log_info "Setting up test project (this may take a moment)..."
        
        # Create minimal project setup
        mkdir -p .localcloud
        
        # Create a basic config file
        cat > .localcloud/config.yaml << EOF
project:
  name: test-components-project
  type: custom
  components: []
services:
  ai:
    port: 0
    models: []
    default: ""
  database:
    type: ""
    version: ""
    port: 0
    extensions: []
  mongodb:
    type: ""
    version: ""
    port: 0
    replicaSet: false
    authEnabled: false
  cache:
    type: ""
    port: 0
    maxMemory: ""
    maxMemoryPolicy: ""
    persistence: false
  queue:
    type: ""
    port: 0
    maxMemory: ""
    maxMemoryPolicy: ""
    persistence: false
    appendOnly: false
    appendFsync: ""
  storage:
    type: ""
    port: 0
    console: 0
  whisper:
    type: ""
    port: 0
EOF
        
        if [[ ! -f ".localcloud/config.yaml" ]]; then
            log_error "Failed to create LocalCloud project configuration"
            return 1
        fi
    fi
    
    log_success "LocalCloud test project ready"
    return 0
}

setup_environment() {
    log_info "Setting up test environment..."
    
    # Setup LocalCloud project first
    if ! setup_test_project; then
        log_error "Failed to setup LocalCloud test project"
        exit 1
    fi
    
    # Create reports directory
    mkdir -p "$REPORTS_DIR"
    
    # Initialize reporter
    init_reporter "$OUTPUT_FORMAT" "$REPORTS_DIR"
    
    # Set verbose mode
    if [[ "$VERBOSE" == "true" ]]; then
        export VERBOSE_MODE=true
    fi
    
    # Set cleanup mode
    export CLEANUP_ON_ERROR="$CLEANUP_ON_ERROR"
    
    log_success "Environment setup complete"
}

run_component_test() {
    local component="$1"
    local test_script="$COMPONENTS_DIR/test-$component.sh"
    
    if [[ ! -f "$test_script" ]]; then
        log_error "Test script not found: $test_script"
        return 1
    fi
    
    if [[ ! -x "$test_script" ]]; then
        log_error "Test script not executable: $test_script"
        return 1
    fi
    
    log_info "Running test for component: $component"
    
    local start_time=$(date +%s)
    local result="PASS"
    local error_message=""
    
    # Run the test with timeout (use different commands for different platforms)
    local test_command=""
    if command -v timeout &>/dev/null; then
        # GNU timeout (Linux)
        test_command="timeout $TIMEOUT bash \"$test_script\" \"$component\""
    elif command -v gtimeout &>/dev/null; then
        # GNU timeout from coreutils (macOS with brew install coreutils)
        test_command="gtimeout $TIMEOUT bash \"$test_script\" \"$component\""
    else
        # Fallback: run without timeout
        log_warning "timeout command not available, running without timeout limit"
        test_command="bash \"$test_script\" \"$component\""
    fi
    
    if eval "$test_command"; then
        result="PASS"
        log_success "Component test passed: $component"
    else
        result="FAIL"
        error_message="Test failed or timed out after ${TIMEOUT}s"
        log_error "Component test failed: $component - $error_message"
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Store results
    TEST_RESULTS["$component"]="$result"
    TEST_DURATIONS["$component"]="$duration"
    
    # Report result
    report_test_result "$component" "$result" "$duration" "$error_message"
    
    return $([[ "$result" == "PASS" ]] && echo 0 || echo 1)
}

run_tests_sequential() {
    log_info "Running tests sequentially..."
    
    local failed_tests=()
    
    for component in "${COMPONENTS[@]}"; do
        if ! run_component_test "$component"; then
            failed_tests+=("$component")
        fi
    done
    
    return ${#failed_tests[@]}
}

run_tests_parallel() {
    log_info "Running tests in parallel (jobs: $PARALLEL_JOBS)..."
    
    local pids=()
    local failed_tests=()
    
    # Run tests in parallel
    for component in "${COMPONENTS[@]}"; do
        # Wait if we've reached the parallel limit
        while [[ ${#pids[@]} -ge $PARALLEL_JOBS ]]; do
            for i in "${!pids[@]}"; do
                if ! kill -0 "${pids[$i]}" 2>/dev/null; then
                    wait "${pids[$i]}"
                    unset pids[$i]
                fi
            done
            pids=("${pids[@]}")  # Reindex array
            sleep 1
        done
        
        # Start test in background
        run_component_test "$component" &
        pids+=($!)
    done
    
    # Wait for all tests to complete
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    # Count failures
    local failure_count=0
    for component in "${COMPONENTS[@]}"; do
        if [[ "${TEST_RESULTS[$component]}" == "FAIL" ]]; then
            ((failure_count++))
        fi
    done
    
    return $failure_count
}

generate_summary() {
    log_info "Generating test summary..."
    
    local total_tests=${#COMPONENTS[@]}
    local passed_tests=0
    local failed_tests=0
    local total_duration=0
    
    for component in "${COMPONENTS[@]}"; do
        if [[ "${TEST_RESULTS[$component]}" == "PASS" ]]; then
            ((passed_tests++))
        else
            ((failed_tests++))
        fi
        total_duration=$((total_duration + TEST_DURATIONS[$component]))
    done
    
    # Console output
    echo
    echo "================================="
    echo "    TEST SUMMARY"
    echo "================================="
    echo "Total tests:    $total_tests"
    echo "Passed:         $passed_tests"
    echo "Failed:         $failed_tests"
    echo "Total duration: ${total_duration}s"
    echo
    
    if [[ $failed_tests -gt 0 ]]; then
        echo "Failed tests:"
        for component in "${COMPONENTS[@]}"; do
            if [[ "${TEST_RESULTS[$component]}" == "FAIL" ]]; then
                echo "  - $component"
            fi
        done
        echo
    fi
    
    # Generate final report
    finalize_report "$total_tests" "$passed_tests" "$failed_tests" "$total_duration"
}

cleanup() {
    log_info "Cleaning up..."
    
    # Stop all services
    if command -v lc &> /dev/null; then
        lc stop &> /dev/null || true
    fi
    
    # Clean up temporary project if we created it
    if [[ -f ".localcloud/config.yaml" ]] && [[ "$CLEANUP_ON_ERROR" == "true" ]]; then
        local project_name
        project_name=$(grep "name:" .localcloud/config.yaml 2>/dev/null | cut -d: -f2 | tr -d ' "' || echo "")
        
        if [[ "$project_name" == "test-components-project" ]]; then
            log_info "Cleaning up temporary test project..."
            rm -rf .localcloud 2>/dev/null || true
        fi
    fi
    
    log_info "Cleanup complete"
}

main() {
    # Parse command line arguments
    parse_args "$@"
    
    # Validate arguments
    validate_args
    
    # Convert components string to array
    IFS=',' read -ra COMPONENTS <<< "$COMPONENTS_TO_TEST"
    
    # Setup
    check_prerequisites
    setup_environment
    
    # Setup signal handlers
    trap cleanup EXIT
    trap 'log_error "Interrupted by user"; exit 130' INT TERM
    
    log_info "Starting LocalCloud component tests..."
    log_info "Components to test: ${COMPONENTS[*]}"
    log_info "Parallel jobs: $PARALLEL_JOBS"
    log_info "Timeout per test: ${TIMEOUT}s"
    log_info "Output format: $OUTPUT_FORMAT"
    echo
    
    # Run tests
    local failure_count=0
    if [[ $PARALLEL_JOBS -gt 1 ]]; then
        run_tests_parallel
        failure_count=$?
    else
        run_tests_sequential
        failure_count=$?
    fi
    
    # Generate summary
    generate_summary
    
    # Exit with appropriate code
    if [[ $failure_count -gt 0 ]]; then
        log_error "Some tests failed"
        exit 1
    else
        log_success "All tests passed!"
        exit 0
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi