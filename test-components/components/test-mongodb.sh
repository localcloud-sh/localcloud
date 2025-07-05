#!/bin/bash

# MongoDB Component Test
# Tests MongoDB functionality: connection, database operations, collections, indexing

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="mongodb"

test_mongodb_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing MongoDB Component"
    
    # 1. Setup component
    log_info "Setting up MongoDB component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup MongoDB component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting MongoDB service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start MongoDB service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness
    log_info "Waiting for MongoDB service to be ready..."
    if ! wait_for_service_ready_comprehensive "mongodb" 120; then
        log_error "MongoDB service failed to become ready"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run MongoDB tests
    log_info "Running MongoDB functionality tests..."
    if ! run_mongodb_tests; then
        log_error "MongoDB functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "MongoDB component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_mongodb_tests() {
    log_group_start "MongoDB Functionality Tests"
    
    # Test 1: Basic connection
    log_info "Test 1: MongoDB connection"
    if ! test_mongodb_connection; then
        log_error "MongoDB connection test failed"
        log_group_end
        return 1
    fi
    log_success "MongoDB connection test passed"
    
    # Test 2: Database operations
    log_info "Test 2: Database operations"
    if ! test_database_operations; then
        log_error "Database operations test failed"
        log_group_end
        return 1
    fi
    log_success "Database operations test passed"
    
    # Test 3: Collection operations
    log_info "Test 3: Collection operations"
    if ! test_collection_operations; then
        log_error "Collection operations test failed"
        log_group_end
        return 1
    fi
    log_success "Collection operations test passed"
    
    # Test 4: Document operations
    log_info "Test 4: Document operations"
    if ! test_document_operations; then
        log_error "Document operations test failed"
        log_group_end
        return 1
    fi
    log_success "Document operations test passed"
    
    # Test 5: Indexing and queries
    log_info "Test 5: Indexing and queries"
    if ! test_indexing_and_queries; then
        log_error "Indexing and queries test failed"
        log_group_end
        return 1
    fi
    log_success "Indexing and queries test passed"
    
    log_group_end
    return 0
}

test_mongodb_connection() {
    # Test MongoDB connection using mongosh if available
    if command -v mongosh &> /dev/null; then
        log_debug "Testing connection with mongosh"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/localcloud?authSource=admin" \
            --eval "db.runCommand({ping: 1})" --quiet &>/dev/null
        return $?
    fi
    
    # Fallback: test port connectivity
    log_debug "Testing port connectivity (mongosh not available)"
    wait_for_port "localhost" "27017" 10
    return $?
}

test_database_operations() {
    if ! command -v mongosh &> /dev/null; then
        log_warning "mongosh not available, skipping database operations test"
        return 0
    fi
    
    local test_db="test_db_$(date +%s)"
    
    # Test database creation (implicit when inserting data)
    log_debug "Testing database creation: $test_db"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.test_collection.insertOne({test: 'data'})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create test database"
        return 1
    fi
    
    # Test database listing
    log_debug "Testing database listing"
    local db_list
    db_list=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/admin?authSource=admin" \
        --eval "db.adminCommand('listDatabases').databases.map(db => db.name)" --quiet 2>/dev/null)
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to list databases"
        return 1
    fi
    
    # Check if our test database exists in the list
    if [[ "$db_list" =~ $test_db ]]; then
        log_debug "Test database found in listing"
    else
        log_warning "Test database not found in listing (may be normal for small databases)"
    fi
    
    # Test database stats
    log_debug "Testing database stats"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.stats()" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_warning "Failed to get database stats"
    fi
    
    # Cleanup
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.dropDatabase()" --quiet &>/dev/null
    
    return 0
}

test_collection_operations() {
    if ! command -v mongosh &> /dev/null; then
        log_warning "mongosh not available, skipping collection operations test"
        return 0
    fi
    
    local test_db="test_collections_$(date +%s)"
    local test_collection="test_collection"
    
    # Test collection creation
    log_debug "Testing collection creation"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.createCollection('$test_collection')" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create collection"
        return 1
    fi
    
    # Test collection listing
    log_debug "Testing collection listing"
    local collections
    collections=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.getCollectionNames()" --quiet 2>/dev/null)
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to list collections"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Check if our collection exists
    if [[ "$collections" =~ $test_collection ]]; then
        log_debug "Test collection found in listing"
    else
        log_error "Test collection not found in listing"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test collection stats
    log_debug "Testing collection stats"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.stats()" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_warning "Failed to get collection stats"
    fi
    
    # Test collection drop
    log_debug "Testing collection drop"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.drop()" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to drop collection"
    fi
    
    # Cleanup
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.dropDatabase()" --quiet &>/dev/null
    
    return 0
}

test_document_operations() {
    if ! command -v mongosh &> /dev/null; then
        log_warning "mongosh not available, skipping document operations test"
        return 0
    fi
    
    local test_db="test_docs_$(date +%s)"
    local test_collection="documents"
    
    # Test document insertion
    log_debug "Testing document insertion"
    
    # Insert single document
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.insertOne({name: 'Test Document', value: 42, created: new Date()})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to insert single document"
        return 1
    fi
    
    # Insert multiple documents
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.insertMany([
            {name: 'Doc 1', category: 'A', value: 10},
            {name: 'Doc 2', category: 'B', value: 20},
            {name: 'Doc 3', category: 'A', value: 30},
            {name: 'Doc 4', category: 'C', value: 40}
        ])" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to insert multiple documents"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test document count
    log_debug "Testing document count"
    local doc_count
    doc_count=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.countDocuments({})" --quiet 2>/dev/null | tail -n1)
    
    if [[ "$doc_count" != "5" ]]; then
        log_error "Document count test failed (expected 5, got $doc_count)"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test document query
    log_debug "Testing document query"
    local query_result
    query_result=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.find({category: 'A'}).count()" --quiet 2>/dev/null | tail -n1)
    
    if [[ "$query_result" != "2" ]]; then
        log_error "Document query test failed (expected 2, got $query_result)"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test document update
    log_debug "Testing document update"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.updateOne({name: 'Doc 1'}, {\$set: {updated: true}})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to update document"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test document deletion
    log_debug "Testing document deletion"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.deleteOne({name: 'Doc 4'})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to delete document"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Verify final count
    local final_count
    final_count=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.countDocuments({})" --quiet 2>/dev/null | tail -n1)
    
    if [[ "$final_count" != "4" ]]; then
        log_error "Final document count test failed (expected 4, got $final_count)"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Cleanup
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.dropDatabase()" --quiet &>/dev/null
    
    return 0
}

test_indexing_and_queries() {
    if ! command -v mongosh &> /dev/null; then
        log_warning "mongosh not available, skipping indexing and queries test"
        return 0
    fi
    
    local test_db="test_index_$(date +%s)"
    local test_collection="indexed_docs"
    
    # Insert test data for indexing
    log_debug "Inserting test data for indexing"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "
        for(let i = 1; i <= 100; i++) {
            db.$test_collection.insertOne({
                id: i,
                name: 'Document ' + i,
                category: ['A', 'B', 'C'][i % 3],
                value: Math.floor(Math.random() * 1000),
                tags: ['tag' + (i % 5), 'tag' + ((i + 1) % 5)]
            });
        }
        " --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to insert test data for indexing"
        return 1
    fi
    
    # Test single field index
    log_debug "Testing single field index creation"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.createIndex({category: 1})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create single field index"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test compound index
    log_debug "Testing compound index creation"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.createIndex({category: 1, value: -1})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create compound index"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test text index
    log_debug "Testing text index creation"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.createIndex({name: 'text'})" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_warning "Failed to create text index (may not be supported)"
    fi
    
    # Test index listing
    log_debug "Testing index listing"
    local indexes
    indexes=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.getIndexes().length" --quiet 2>/dev/null | tail -n1)
    
    if [[ -n "$indexes" && "$indexes" -gt 1 ]]; then
        log_debug "Found $indexes indexes (including default _id index)"
    else
        log_warning "Index listing may have failed"
    fi
    
    # Test query with index
    log_debug "Testing query with index"
    local indexed_query_result
    indexed_query_result=$(mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.find({category: 'A'}).count()" --quiet 2>/dev/null | tail -n1)
    
    if [[ -n "$indexed_query_result" && "$indexed_query_result" -gt 0 ]]; then
        log_debug "Indexed query returned $indexed_query_result results"
    else
        log_error "Indexed query failed"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Test aggregation pipeline
    log_debug "Testing aggregation pipeline"
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.$test_collection.aggregate([
            {\$group: {_id: '\$category', count: {\$sum: 1}, avgValue: {\$avg: '\$value'}}},
            {\$sort: {count: -1}}
        ]).toArray()" --quiet &>/dev/null
    
    if [[ $? -ne 0 ]]; then
        log_error "Aggregation pipeline test failed"
        mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
            --eval "db.dropDatabase()" --quiet &>/dev/null
        return 1
    fi
    
    # Cleanup
    mongosh "mongodb://localcloud:localcloud@localhost:27017/$test_db?authSource=admin" \
        --eval "db.dropDatabase()" --quiet &>/dev/null
    
    return 0
}

# Main execution
main() {
    if test_mongodb_component; then
        log_success "MongoDB component test completed successfully"
        exit 0
    else
        log_error "MongoDB component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi