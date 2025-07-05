#!/bin/bash

# Redis Cache Component Test
# Tests Redis cache functionality: connection, key-value operations, expiration

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="cache"

test_cache_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing Redis Cache Component"
    
    # 1. Setup component
    log_info "Setting up cache component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup cache component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting cache service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start cache service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness
    log_info "Waiting for cache service to be ready..."
    if ! wait_for_service_ready_comprehensive "cache" 60; then
        log_error "Cache service failed to become ready"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run cache tests
    log_info "Running cache functionality tests..."
    if ! run_cache_tests; then
        log_error "Cache functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "Cache component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_cache_tests() {
    log_group_start "Cache Functionality Tests"
    
    # Test 1: Basic connection
    log_info "Test 1: Cache connection"
    if ! test_cache_connection; then
        log_error "Cache connection test failed"
        log_group_end
        return 1
    fi
    log_success "Cache connection test passed"
    
    # Test 2: Basic key-value operations
    log_info "Test 2: Key-value operations"
    if ! test_key_value_operations; then
        log_error "Key-value operations test failed"
        log_group_end
        return 1
    fi
    log_success "Key-value operations test passed"
    
    # Test 3: Expiration functionality
    log_info "Test 3: Key expiration"
    if ! test_key_expiration; then
        log_error "Key expiration test failed"
        log_group_end
        return 1
    fi
    log_success "Key expiration test passed"
    
    # Test 4: Data types
    log_info "Test 4: Redis data types"
    if ! test_redis_data_types; then
        log_error "Redis data types test failed"
        log_group_end
        return 1
    fi
    log_success "Redis data types test passed"
    
    # Test 5: Performance test
    log_info "Test 5: Basic performance test"
    if ! test_cache_performance; then
        log_error "Cache performance test failed"
        log_group_end
        return 1
    fi
    log_success "Cache performance test passed"
    
    log_group_end
    return 0
}

test_cache_connection() {
    # Test Redis connection using redis-cli if available
    if command -v redis-cli &> /dev/null; then
        log_debug "Testing connection with redis-cli"
        redis-cli -h localhost -p 6379 ping &>/dev/null
        return $?
    fi
    
    # Fallback: test port connectivity
    log_debug "Testing port connectivity (redis-cli not available)"
    wait_for_port "localhost" "6379" 10
    return $?
}

test_key_value_operations() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping key-value operations test"
        return 0
    fi
    
    local test_key="test:component:$(date +%s)"
    local test_value="test_value_$(date +%s)"
    
    # Set key
    log_debug "Setting key: $test_key"
    redis-cli -h localhost -p 6379 SET "$test_key" "$test_value" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to set key in Redis"
        return 1
    fi
    
    # Get key
    log_debug "Getting key: $test_key"
    local retrieved_value
    retrieved_value=$(redis-cli -h localhost -p 6379 GET "$test_key" 2>/dev/null)
    
    if [[ "$retrieved_value" != "$test_value" ]]; then
        log_error "Key-value mismatch (expected: $test_value, got: $retrieved_value)"
        redis-cli -h localhost -p 6379 DEL "$test_key" &>/dev/null
        return 1
    fi
    
    # Check if key exists
    log_debug "Checking if key exists"
    local exists_result
    exists_result=$(redis-cli -h localhost -p 6379 EXISTS "$test_key" 2>/dev/null)
    
    if [[ "$exists_result" != "1" ]]; then
        log_error "Key existence check failed"
        redis-cli -h localhost -p 6379 DEL "$test_key" &>/dev/null
        return 1
    fi
    
    # Delete key
    log_debug "Deleting key"
    redis-cli -h localhost -p 6379 DEL "$test_key" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to delete key from Redis"
        return 1
    fi
    
    # Verify key is deleted
    local exists_after_delete
    exists_after_delete=$(redis-cli -h localhost -p 6379 EXISTS "$test_key" 2>/dev/null)
    
    if [[ "$exists_after_delete" != "0" ]]; then
        log_error "Key deletion verification failed"
        return 1
    fi
    
    return 0
}

test_key_expiration() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping key expiration test"
        return 0
    fi
    
    local test_key="test:expire:$(date +%s)"
    local test_value="expire_value"
    
    # Set key with expiration (5 seconds)
    log_debug "Setting key with 5s expiration: $test_key"
    redis-cli -h localhost -p 6379 SETEX "$test_key" 5 "$test_value" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to set key with expiration"
        return 1
    fi
    
    # Verify key exists
    local initial_value
    initial_value=$(redis-cli -h localhost -p 6379 GET "$test_key" 2>/dev/null)
    
    if [[ "$initial_value" != "$test_value" ]]; then
        log_error "Key with expiration not set correctly"
        redis-cli -h localhost -p 6379 DEL "$test_key" &>/dev/null
        return 1
    fi
    
    # Check TTL
    local ttl
    ttl=$(redis-cli -h localhost -p 6379 TTL "$test_key" 2>/dev/null)
    
    if [[ "$ttl" -le 0 ]]; then
        log_error "TTL check failed (expected > 0, got: $ttl)"
        redis-cli -h localhost -p 6379 DEL "$test_key" &>/dev/null
        return 1
    fi
    
    log_debug "Key TTL: ${ttl}s"
    
    # Wait for key to expire (wait a bit longer than TTL)
    log_debug "Waiting for key to expire..."
    sleep 6
    
    # Verify key has expired
    local expired_value
    expired_value=$(redis-cli -h localhost -p 6379 GET "$test_key" 2>/dev/null)
    
    if [[ -n "$expired_value" ]]; then
        log_error "Key did not expire as expected (value: $expired_value)"
        redis-cli -h localhost -p 6379 DEL "$test_key" &>/dev/null
        return 1
    fi
    
    return 0
}

test_redis_data_types() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping data types test"
        return 0
    fi
    
    local test_prefix="test:types:$(date +%s)"
    
    # Test Hash
    log_debug "Testing Hash data type"
    local hash_key="${test_prefix}:hash"
    redis-cli -h localhost -p 6379 HSET "$hash_key" field1 value1 field2 value2 &>/dev/null
    
    local hash_value
    hash_value=$(redis-cli -h localhost -p 6379 HGET "$hash_key" field1 2>/dev/null)
    
    if [[ "$hash_value" != "value1" ]]; then
        log_error "Hash data type test failed"
        redis-cli -h localhost -p 6379 DEL "$hash_key" &>/dev/null
        return 1
    fi
    
    # Test List
    log_debug "Testing List data type"
    local list_key="${test_prefix}:list"
    redis-cli -h localhost -p 6379 LPUSH "$list_key" item1 item2 item3 &>/dev/null
    
    local list_length
    list_length=$(redis-cli -h localhost -p 6379 LLEN "$list_key" 2>/dev/null)
    
    if [[ "$list_length" != "3" ]]; then
        log_error "List data type test failed (expected length 3, got $list_length)"
        redis-cli -h localhost -p 6379 DEL "$list_key" &>/dev/null
        return 1
    fi
    
    # Test Set
    log_debug "Testing Set data type"
    local set_key="${test_prefix}:set"
    redis-cli -h localhost -p 6379 SADD "$set_key" member1 member2 member3 &>/dev/null
    
    local set_size
    set_size=$(redis-cli -h localhost -p 6379 SCARD "$set_key" 2>/dev/null)
    
    if [[ "$set_size" != "3" ]]; then
        log_error "Set data type test failed (expected size 3, got $set_size)"
        redis-cli -h localhost -p 6379 DEL "$set_key" &>/dev/null
        return 1
    fi
    
    # Cleanup
    redis-cli -h localhost -p 6379 DEL "$hash_key" "$list_key" "$set_key" &>/dev/null
    
    return 0
}

test_cache_performance() {
    if ! command -v redis-cli &> /dev/null; then
        log_warning "redis-cli not available, skipping performance test"
        return 0
    fi
    
    local test_prefix="test:perf:$(date +%s)"
    local num_operations=1000
    
    # Measure set performance
    local start_time=$(get_epoch)
    log_debug "Running performance test ($num_operations SET operations)"
    
    for i in $(seq 1 $num_operations); do
        redis-cli -h localhost -p 6379 SET "${test_prefix}:key_$i" "value_$i" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Performance test failed at SET operation $i"
            # Cleanup partial data
            redis-cli -h localhost -p 6379 --scan --pattern "${test_prefix}:*" | xargs redis-cli DEL &>/dev/null
            return 1
        fi
    done
    
    local set_end_time=$(get_epoch)
    local set_duration=$((set_end_time - start_time))
    
    # Measure get performance
    local get_start_time=$(get_epoch)
    log_debug "Running performance test ($num_operations GET operations)"
    
    for i in $(seq 1 $num_operations); do
        local value
        value=$(redis-cli -h localhost -p 6379 GET "${test_prefix}:key_$i" 2>/dev/null)
        
        if [[ "$value" != "value_$i" ]]; then
            log_error "Performance test failed at GET operation $i (expected: value_$i, got: $value)"
            # Cleanup
            redis-cli -h localhost -p 6379 --scan --pattern "${test_prefix}:*" | xargs redis-cli DEL &>/dev/null
            return 1
        fi
    done
    
    local get_end_time=$(get_epoch)
    local get_duration=$((get_end_time - get_start_time))
    
    log_debug "Performance test completed: $num_operations SETs in ${set_duration}s, $num_operations GETs in ${get_duration}s"
    
    # Performance threshold checks
    if [[ $set_duration -gt 10 ]]; then
        log_warning "SET performance slow: ${set_duration}s for $num_operations operations"
    fi
    
    if [[ $get_duration -gt 5 ]]; then
        log_warning "GET performance slow: ${get_duration}s for $num_operations operations"
    fi
    
    # Cleanup
    redis-cli -h localhost -p 6379 --scan --pattern "${test_prefix}:*" | xargs redis-cli DEL &>/dev/null
    
    return 0
}

# Main execution
main() {
    if test_cache_component; then
        log_success "Redis cache component test completed successfully"
        exit 0
    else
        log_error "Redis cache component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi