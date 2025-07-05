#!/bin/bash

# PostgreSQL Database Component Test
# Tests basic database functionality: connection, table creation, data operations

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="database"

test_database_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing PostgreSQL Database Component"
    
    # 1. Setup component
    log_info "Setting up database component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup database component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting database service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start database service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness
    log_info "Waiting for database service to be ready..."
    if ! wait_for_service_ready_comprehensive "postgres" 120; then
        log_error "Database service failed to become ready"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run database tests
    log_info "Running database functionality tests..."
    if ! run_database_tests; then
        log_error "Database functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "Database component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_database_tests() {
    log_group_start "Database Functionality Tests"
    
    # Test 1: Basic connection
    log_info "Test 1: Database connection"
    if ! test_database_connection; then
        log_error "Database connection test failed"
        log_group_end
        return 1
    fi
    log_success "Database connection test passed"
    
    # Test 2: Table operations
    log_info "Test 2: Table operations"
    if ! test_table_operations; then
        log_error "Table operations test failed"
        log_group_end
        return 1
    fi
    log_success "Table operations test passed"
    
    # Test 3: Data operations
    log_info "Test 3: Data operations"
    if ! test_data_operations; then
        log_error "Data operations test failed"
        log_group_end
        return 1
    fi
    log_success "Data operations test passed"
    
    # Test 4: Performance test
    log_info "Test 4: Basic performance test"
    if ! test_database_performance; then
        log_error "Database performance test failed"
        log_group_end
        return 1
    fi
    log_success "Database performance test passed"
    
    log_group_end
    return 0
}

test_database_connection() {
    # Test PostgreSQL connection using psql if available
    if command -v psql &> /dev/null; then
        log_debug "Testing connection with psql"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud -c "SELECT version();" &>/dev/null
        return $?
    fi
    
    # Fallback: test port connectivity
    log_debug "Testing port connectivity (psql not available)"
    wait_for_port "localhost" "5432" 10
    return $?
}

test_table_operations() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping table operations test"
        return 0
    fi
    
    local test_table="test_component_$(date +%s)"
    
    # Create table
    log_debug "Creating test table: $test_table"
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE TABLE $test_table (id SERIAL PRIMARY KEY, name VARCHAR(100), created_at TIMESTAMP DEFAULT NOW());" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create test table"
        return 1
    fi
    
    # Verify table exists
    log_debug "Verifying table exists"
    local table_count
    table_count=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='$test_table';" 2>/dev/null | tr -d ' ')
    
    if [[ "$table_count" != "1" ]]; then
        log_error "Table verification failed"
        return 1
    fi
    
    # Drop table
    log_debug "Dropping test table"
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DROP TABLE $test_table;" &>/dev/null
    
    return $?
}

test_data_operations() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping data operations test"
        return 0
    fi
    
    local test_table="test_data_$(date +%s)"
    
    # Create test table
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE TABLE $test_table (id SERIAL PRIMARY KEY, name VARCHAR(100), value INTEGER);" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create test table for data operations"
        return 1
    fi
    
    # Insert test data
    log_debug "Inserting test data"
    for i in {1..5}; do
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "INSERT INTO $test_table (name, value) VALUES ('test_$i', $i);" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Failed to insert test data (record $i)"
            PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
                -c "DROP TABLE $test_table;" &>/dev/null
            return 1
        fi
    done
    
    # Query data
    log_debug "Querying test data"
    local record_count
    record_count=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT COUNT(*) FROM $test_table;" 2>/dev/null | tr -d ' ')
    
    if [[ "$record_count" != "5" ]]; then
        log_error "Data query test failed (expected 5 records, got $record_count)"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Update data
    log_debug "Updating test data"
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "UPDATE $test_table SET value = value * 10 WHERE id <= 3;" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to update test data"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Delete data
    log_debug "Deleting test data"
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DELETE FROM $test_table WHERE id > 3;" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to delete test data"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Verify final state
    local final_count
    final_count=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT COUNT(*) FROM $test_table;" 2>/dev/null | tr -d ' ')
    
    if [[ "$final_count" != "3" ]]; then
        log_error "Final data verification failed (expected 3 records, got $final_count)"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Cleanup
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DROP TABLE $test_table;" &>/dev/null
    
    return 0
}

test_database_performance() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping performance test"
        return 0
    fi
    
    local test_table="test_perf_$(date +%s)"
    
    # Create test table
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE TABLE $test_table (id SERIAL PRIMARY KEY, data TEXT);" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create performance test table"
        return 1
    fi
    
    # Measure insert performance
    local start_time=$(get_epoch)
    log_debug "Running performance test (100 inserts)"
    
    for i in {1..100}; do
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "INSERT INTO $test_table (data) VALUES ('performance_test_data_$i');" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Performance test failed at insert $i"
            PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
                -c "DROP TABLE $test_table;" &>/dev/null
            return 1
        fi
    done
    
    local end_time=$(get_epoch)
    local duration=$((end_time - start_time))
    
    log_debug "Performance test completed: 100 inserts in ${duration}s"
    
    # Performance threshold check (should complete within reasonable time)
    if [[ $duration -gt 30 ]]; then
        log_warning "Performance test slow: ${duration}s (expected < 30s)"
    fi
    
    # Cleanup
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DROP TABLE $test_table;" &>/dev/null
    
    return 0
}

# Main execution
main() {
    if test_database_component; then
        log_success "PostgreSQL database component test completed successfully"
        exit 0
    else
        log_error "PostgreSQL database component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi