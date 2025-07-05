#!/bin/bash

# Redis Queue Component Test
# Tests Redis job queue functionality: job queuing, processing, blocking operations

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="queue"

test_queue_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing Redis Queue Component"
    
    # 1. Setup component
    log_info "Setting up queue component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup queue component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting queue service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start queue service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness
    log_info "Waiting for queue service to be ready..."
    if ! wait_for_service_ready_comprehensive "queue" 60; then
        log_error "Queue service failed to become ready"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run queue tests
    log_info "Running queue functionality tests..."
    if ! run_queue_tests; then
        log_error "Queue functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "Queue component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_queue_tests() {
    log_group_start "Queue Functionality Tests"
    
    # Test 1: Basic connection
    log_info "Test 1: Queue connection"
    if ! test_queue_connection; then
        log_error "Queue connection test failed"
        log_group_end
        return 1
    fi
    log_success "Queue connection test passed"
    
    # Test 2: Basic queue operations
    log_info "Test 2: Basic queue operations"
    if ! test_basic_queue_operations; then
        log_error "Basic queue operations test failed"
        log_group_end
        return 1
    fi
    log_success "Basic queue operations test passed"
    
    # Test 3: Job processing patterns
    log_info "Test 3: Job processing patterns"
    if ! test_job_processing_patterns; then
        log_error "Job processing patterns test failed"
        log_group_end
        return 1
    fi
    log_success "Job processing patterns test passed"
    
    # Test 4: Queue persistence
    log_info "Test 4: Queue persistence"
    if ! test_queue_persistence; then
        log_error "Queue persistence test failed"
        log_group_end
        return 1
    fi
    log_success "Queue persistence test passed"
    
    # Test 5: Performance test
    log_info "Test 5: Queue performance"
    if ! test_queue_performance; then
        log_error "Queue performance test failed"
        log_group_end
        return 1
    fi
    log_success "Queue performance test passed"
    
    log_group_end
    return 0
}

test_queue_connection() {
    # Test Redis queue connection (port 6380)
    if command -v redis-cli &> /dev/null; then
        log_debug "Testing connection with redis-cli"
        redis-cli -h localhost -p 6380 ping &>/dev/null
        return $?
    fi
    
    # Fallback: test port connectivity
    log_debug "Testing port connectivity (redis-cli not available)"
    wait_for_port "localhost" "6380" 10
    return $?
}

test_basic_queue_operations() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping basic queue operations test"
        return 0
    fi
    
    local queue_name="test:queue:$(date +%s)"
    
    # Test LPUSH (add job to queue)
    log_debug "Testing LPUSH (add job to queue)"
    local job_data='{"id":1,"task":"test_job","data":"test_data"}'
    redis-cli -h localhost -p 6380 LPUSH "$queue_name" "$job_data" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to add job to queue"
        return 1
    fi
    
    # Test LLEN (check queue length)
    log_debug "Testing LLEN (check queue length)"
    local queue_length
    queue_length=$(redis-cli -h localhost -p 6380 LLEN "$queue_name" 2>/dev/null)
    
    if [[ "$queue_length" != "1" ]]; then
        log_error "Queue length check failed (expected 1, got $queue_length)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Test RPOP (remove job from queue)
    log_debug "Testing RPOP (remove job from queue)"
    local retrieved_job
    retrieved_job=$(redis-cli -h localhost -p 6380 RPOP "$queue_name" 2>/dev/null)
    
    if [[ "$retrieved_job" != "$job_data" ]]; then
        log_error "Job retrieval failed (expected: $job_data, got: $retrieved_job)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Verify queue is empty
    queue_length=$(redis-cli -h localhost -p 6380 LLEN "$queue_name" 2>/dev/null)
    if [[ "$queue_length" != "0" ]]; then
        log_error "Queue should be empty after RPOP (length: $queue_length)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Cleanup
    redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
    
    return 0
}

test_job_processing_patterns() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping job processing patterns test"
        return 0
    fi
    
    local queue_name="test:jobs:$(date +%s)"
    local processing_queue="${queue_name}:processing"
    
    # Test 1: FIFO (First In, First Out) processing
    log_debug "Testing FIFO job processing"
    
    # Add multiple jobs
    for i in {1..5}; do
        local job_data="{\"id\":$i,\"task\":\"job_$i\"}"
        redis-cli -h localhost -p 6380 LPUSH "$queue_name" "$job_data" &>/dev/null
    done
    
    # Process jobs in FIFO order
    local processed_jobs=()
    for i in {1..5}; do
        local job
        job=$(redis-cli -h localhost -p 6380 RPOP "$queue_name" 2>/dev/null)
        if [[ -n "$job" ]]; then
            processed_jobs+=("$job")
        fi
    done
    
    # Verify we processed all jobs
    if [[ ${#processed_jobs[@]} -ne 5 ]]; then
        log_error "FIFO processing failed (processed ${#processed_jobs[@]} jobs, expected 5)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Test 2: Blocking pop (BRPOP)
    log_debug "Testing blocking pop (BRPOP)"
    
    # Add a job
    local test_job='{"id":999,"task":"blocking_test"}'
    redis-cli -h localhost -p 6380 LPUSH "$queue_name" "$test_job" &>/dev/null
    
    # Use BRPOP with timeout
    local blocked_job
    blocked_job=$(redis-cli -h localhost -p 6380 BRPOP "$queue_name" 5 2>/dev/null | tail -n1)
    
    if [[ "$blocked_job" != "$test_job" ]]; then
        log_error "Blocking pop failed (expected: $test_job, got: $blocked_job)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Test 3: Reliable queue pattern (RPOPLPUSH)
    log_debug "Testing reliable queue pattern (RPOPLPUSH)"
    
    # Add a job
    local reliable_job='{"id":888,"task":"reliable_test"}'
    redis-cli -h localhost -p 6380 LPUSH "$queue_name" "$reliable_job" &>/dev/null
    
    # Move job to processing queue
    local moved_job
    moved_job=$(redis-cli -h localhost -p 6380 RPOPLPUSH "$queue_name" "$processing_queue" 2>/dev/null)
    
    if [[ "$moved_job" != "$reliable_job" ]]; then
        log_error "Reliable queue pattern failed (RPOPLPUSH)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" "$processing_queue" &>/dev/null
        return 1
    fi
    
    # Verify job is in processing queue
    local processing_length
    processing_length=$(redis-cli -h localhost -p 6380 LLEN "$processing_queue" 2>/dev/null)
    
    if [[ "$processing_length" != "1" ]]; then
        log_error "Job not found in processing queue (length: $processing_length)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" "$processing_queue" &>/dev/null
        return 1
    fi
    
    # Simulate job completion (remove from processing queue)
    redis-cli -h localhost -p 6380 LREM "$processing_queue" 1 "$reliable_job" &>/dev/null
    
    # Cleanup
    redis-cli -h localhost -p 6380 DEL "$queue_name" "$processing_queue" &>/dev/null
    
    return 0
}

test_queue_persistence() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping queue persistence test"
        return 0
    fi
    
    local queue_name="test:persist:$(date +%s)"
    
    # Add persistent jobs
    log_debug "Testing queue persistence"
    
    local persistent_jobs=(
        '{"id":1,"task":"persistent_job_1","priority":"high"}'
        '{"id":2,"task":"persistent_job_2","priority":"medium"}'
        '{"id":3,"task":"persistent_job_3","priority":"low"}'
    )
    
    for job in "${persistent_jobs[@]}"; do
        redis-cli -h localhost -p 6380 LPUSH "$queue_name" "$job" &>/dev/null
        if [[ $? -ne 0 ]]; then
            log_error "Failed to add persistent job to queue"
            redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
            return 1
        fi
    done
    
    # Check persistence configuration
    log_debug "Checking Redis persistence configuration"
    local save_config
    save_config=$(redis-cli -h localhost -p 6380 CONFIG GET save 2>/dev/null | tail -n1)
    
    if [[ -n "$save_config" ]]; then
        log_debug "Redis save configuration: $save_config"
    fi
    
    # Verify jobs are still in queue
    local final_length
    final_length=$(redis-cli -h localhost -p 6380 LLEN "$queue_name" 2>/dev/null)
    
    if [[ "$final_length" != "3" ]]; then
        log_error "Persistence test failed (expected 3 jobs, found $final_length)"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Force save (if supported)
    redis-cli -h localhost -p 6380 BGSAVE &>/dev/null || log_debug "BGSAVE not available or failed"
    
    # Cleanup
    redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
    
    return 0
}

test_queue_performance() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping queue performance test"
        return 0
    fi
    
    local queue_name="test:perf:$(date +%s)"
    local num_jobs=1000
    
    # Test job insertion performance
    log_debug "Testing job insertion performance ($num_jobs jobs)"
    local start_time=$(get_epoch)
    
    for i in $(seq 1 $num_jobs); do
        local job_data="{\"id\":$i,\"task\":\"perf_job_$i\",\"timestamp\":$(date +%s)}"
        redis-cli -h localhost -p 6380 LPUSH "$queue_name" "$job_data" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Performance test failed at job insertion $i"
            redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
            return 1
        fi
    done
    
    local insertion_end_time=$(get_epoch)
    local insertion_duration=$((insertion_end_time - start_time))
    
    # Verify all jobs were inserted
    local queue_length
    queue_length=$(redis-cli -h localhost -p 6380 LLEN "$queue_name" 2>/dev/null)
    
    if [[ "$queue_length" != "$num_jobs" ]]; then
        log_error "Performance test: expected $num_jobs jobs, found $queue_length"
        redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
        return 1
    fi
    
    # Test job processing performance
    log_debug "Testing job processing performance ($num_jobs jobs)"
    local processing_start_time=$(get_epoch)
    
    local processed_count=0
    while [[ $processed_count -lt $num_jobs ]]; do
        local job
        job=$(redis-cli -h localhost -p 6380 RPOP "$queue_name" 2>/dev/null)
        
        if [[ -n "$job" ]]; then
            ((processed_count++))
        else
            break
        fi
    done
    
    local processing_end_time=$(get_epoch)
    local processing_duration=$((processing_end_time - processing_start_time))
    
    log_debug "Performance results:"
    log_debug "  Insertion: $num_jobs jobs in ${insertion_duration}s ($(( num_jobs / insertion_duration )) jobs/sec)"
    log_debug "  Processing: $processed_count jobs in ${processing_duration}s ($(( processed_count / processing_duration )) jobs/sec)"
    
    # Performance thresholds
    if [[ $insertion_duration -gt 15 ]]; then
        log_warning "Job insertion performance slow: ${insertion_duration}s for $num_jobs jobs"
    fi
    
    if [[ $processing_duration -gt 10 ]]; then
        log_warning "Job processing performance slow: ${processing_duration}s for $num_jobs jobs"
    fi
    
    # Verify queue is empty
    local final_length
    final_length=$(redis-cli -h localhost -p 6380 LLEN "$queue_name" 2>/dev/null)
    
    if [[ "$final_length" != "0" ]]; then
        log_warning "Queue not empty after processing (remaining: $final_length jobs)"
    fi
    
    # Cleanup
    redis-cli -h localhost -p 6380 DEL "$queue_name" &>/dev/null
    
    return 0
}

# Main execution
main() {
    if test_queue_component; then
        log_success "Redis queue component test completed successfully"
        exit 0
    else
        log_error "Redis queue component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi