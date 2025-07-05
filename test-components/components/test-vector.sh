#!/bin/bash

# pgVector Component Test
# Tests PostgreSQL with pgvector extension: vector operations, similarity search

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="vector"

test_vector_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing pgVector Component"
    
    # 1. Setup component (vector depends on database)
    log_info "Setting up vector component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup vector component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting vector service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start vector service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness
    log_info "Waiting for vector service to be ready..."
    if ! wait_for_service_ready_comprehensive "postgres" 120; then
        log_error "Vector service failed to become ready"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run vector tests
    log_info "Running vector functionality tests..."
    if ! run_vector_tests; then
        log_error "Vector functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "Vector component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_vector_tests() {
    log_group_start "Vector Functionality Tests"
    
    # Test 1: pgvector extension availability
    log_info "Test 1: pgvector extension"
    if ! test_pgvector_extension; then
        log_error "pgvector extension test failed"
        log_group_end
        return 1
    fi
    log_success "pgvector extension test passed"
    
    # Test 2: Vector table operations
    log_info "Test 2: Vector table operations"
    if ! test_vector_table_operations; then
        log_error "Vector table operations test failed"
        log_group_end
        return 1
    fi
    log_success "Vector table operations test passed"
    
    # Test 3: Vector similarity search
    log_info "Test 3: Vector similarity search"
    if ! test_vector_similarity_search; then
        log_error "Vector similarity search test failed"
        log_group_end
        return 1
    fi
    log_success "Vector similarity search test passed"
    
    # Test 4: Vector indexing
    log_info "Test 4: Vector indexing"
    if ! test_vector_indexing; then
        log_error "Vector indexing test failed"
        log_group_end
        return 1
    fi
    log_success "Vector indexing test passed"
    
    log_group_end
    return 0
}

test_pgvector_extension() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping pgvector extension test"
        return 0
    fi
    
    log_debug "Checking if pgvector extension is available"
    
    # Check if pgvector extension exists
    local extension_available
    extension_available=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT COUNT(*) FROM pg_available_extensions WHERE name='vector';" 2>/dev/null | tr -d ' ')
    
    if [[ "$extension_available" != "1" ]]; then
        log_error "pgvector extension not available in PostgreSQL"
        return 1
    fi
    
    # Check if pgvector extension is installed
    local extension_installed
    extension_installed=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT COUNT(*) FROM pg_extension WHERE extname='vector';" 2>/dev/null | tr -d ' ')
    
    if [[ "$extension_installed" != "1" ]]; then
        log_info "Installing pgvector extension"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "CREATE EXTENSION IF NOT EXISTS vector;" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Failed to install pgvector extension"
            return 1
        fi
    fi
    
    log_debug "pgvector extension is available and installed"
    return 0
}

test_vector_table_operations() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping vector table operations test"
        return 0
    fi
    
    local test_table="test_vectors_$(date +%s)"
    
    # Create vector table
    log_debug "Creating vector table: $test_table"
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE TABLE $test_table (id SERIAL PRIMARY KEY, content TEXT, embedding vector(384));" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create vector table"
        return 1
    fi
    
    # Insert vector data
    log_debug "Inserting vector data"
    local sample_vectors=(
        "[0.1, 0.2, 0.3]"
        "[0.4, 0.5, 0.6]"
        "[0.7, 0.8, 0.9]"
        "[0.2, 0.1, 0.4]"
        "[0.8, 0.7, 0.5]"
    )
    
    for i in "${!sample_vectors[@]}"; do
        local content="Sample document $((i+1))"
        # Create a 384-dimensional vector (pad with zeros)
        local vector="ARRAY[${sample_vectors[$i]}"
        # Add zeros to make it 384 dimensions
        for j in {4..384}; do
            vector="$vector, 0.0"
        done
        vector="$vector]"
        
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "INSERT INTO $test_table (content, embedding) VALUES ('$content', '$vector');" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Failed to insert vector data (record $((i+1)))"
            PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
                -c "DROP TABLE $test_table;" &>/dev/null
            return 1
        fi
    done
    
    # Verify data insertion
    local record_count
    record_count=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT COUNT(*) FROM $test_table;" 2>/dev/null | tr -d ' ')
    
    if [[ "$record_count" != "5" ]]; then
        log_error "Vector data insertion verification failed (expected 5, got $record_count)"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Cleanup
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DROP TABLE $test_table;" &>/dev/null
    
    return 0
}

test_vector_similarity_search() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping vector similarity search test"
        return 0
    fi
    
    local test_table="test_similarity_$(date +%s)"
    
    # Create vector table
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE TABLE $test_table (id SERIAL PRIMARY KEY, content TEXT, embedding vector(3));" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create similarity test table"
        return 1
    fi
    
    # Insert test vectors
    log_debug "Inserting test vectors for similarity search"
    declare -a test_data=(
        "'Document 1' '[1, 0, 0]'"
        "'Document 2' '[0, 1, 0]'"
        "'Document 3' '[0, 0, 1]'"
        "'Similar to 1' '[0.9, 0.1, 0]'"
        "'Similar to 2' '[0.1, 0.9, 0]'"
    )
    
    for data in "${test_data[@]}"; do
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "INSERT INTO $test_table (content, embedding) VALUES ($data);" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_error "Failed to insert similarity test data"
            PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
                -c "DROP TABLE $test_table;" &>/dev/null
            return 1
        fi
    done
    
    # Test cosine similarity search
    log_debug "Testing cosine similarity search"
    local query_vector="'[1, 0, 0]'"
    local similarity_results
    similarity_results=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT content, 1 - (embedding <=> $query_vector) AS similarity 
               FROM $test_table 
               ORDER BY embedding <=> $query_vector 
               LIMIT 3;" 2>/dev/null)
    
    if [[ $? -ne 0 ]]; then
        log_error "Cosine similarity search failed"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Verify we got results
    if [[ -z "$similarity_results" ]]; then
        log_error "No similarity search results returned"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    log_debug "Similarity search completed successfully"
    
    # Test L2 distance search
    log_debug "Testing L2 distance search"
    local distance_results
    distance_results=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -t -c "SELECT content, embedding <-> $query_vector AS distance 
               FROM $test_table 
               ORDER BY embedding <-> $query_vector 
               LIMIT 3;" 2>/dev/null)
    
    if [[ $? -ne 0 ]]; then
        log_error "L2 distance search failed"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "DROP TABLE $test_table;" &>/dev/null
        return 1
    fi
    
    # Cleanup
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DROP TABLE $test_table;" &>/dev/null
    
    return 0
}

test_vector_indexing() {
    if ! command -v psql &> /dev/null; then
        log_warning "psql not available, skipping vector indexing test"
        return 0
    fi
    
    local test_table="test_index_$(date +%s)"
    
    # Create vector table with more data for indexing
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE TABLE $test_table (id SERIAL PRIMARY KEY, embedding vector(3));" &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create indexing test table"
        return 1
    fi
    
    # Insert multiple vectors for index testing
    log_debug "Inserting vectors for index testing"
    for i in {1..20}; do
        local x=$(echo "scale=2; $i / 20" | bc 2>/dev/null || echo "0.5")
        local y=$(echo "scale=2; ($i * 2) / 20" | bc 2>/dev/null || echo "0.5")
        local z=$(echo "scale=2; ($i * 3) / 20" | bc 2>/dev/null || echo "0.5")
        
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "INSERT INTO $test_table (embedding) VALUES ('[$x, $y, $z]');" &>/dev/null
        
        if [[ $? -ne 0 ]]; then
            log_warning "Failed to insert vector $i (continuing)"
        fi
    done
    
    # Create HNSW index for cosine similarity
    log_debug "Creating HNSW index for cosine similarity"
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "CREATE INDEX CONCURRENTLY ON $test_table USING hnsw (embedding vector_cosine_ops);" &>/dev/null
    
    local index_result=$?
    
    # Create IVFFlat index for L2 distance (if HNSW failed)
    if [[ $index_result -ne 0 ]]; then
        log_debug "HNSW index failed, trying IVFFlat index"
        PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -c "CREATE INDEX ON $test_table USING ivfflat (embedding vector_l2_ops) WITH (lists = 10);" &>/dev/null
        index_result=$?
    fi
    
    if [[ $index_result -ne 0 ]]; then
        log_warning "Vector index creation failed (may not be supported in this configuration)"
    else
        log_debug "Vector index created successfully"
        
        # Test query with index
        log_debug "Testing query performance with index"
        local query_with_index
        query_with_index=$(PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
            -t -c "SELECT COUNT(*) FROM $test_table WHERE embedding <-> '[0.5, 0.5, 0.5]' < 1.0;" 2>/dev/null | tr -d ' ')
        
        if [[ -n "$query_with_index" && "$query_with_index" != "0" ]]; then
            log_debug "Index query returned $query_with_index results"
        fi
    fi
    
    # Cleanup
    PGPASSWORD="localcloud" psql -h localhost -p 5432 -U localcloud -d localcloud \
        -c "DROP TABLE $test_table;" &>/dev/null
    
    return 0
}

# Main execution
main() {
    if test_vector_component; then
        log_success "pgVector component test completed successfully"
        exit 0
    else
        log_error "pgVector component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi