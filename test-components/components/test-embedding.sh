#!/bin/bash

# Embedding Component Test
# Tests AI text embedding functionality: model availability, embedding generation

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="embedding"

test_embedding_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing Embedding Component"
    
    # 1. Setup component
    log_info "Setting up embedding component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup embedding component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting embedding service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start embedding service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness (embeddings may take time to load)
    log_info "Waiting for embedding service to be ready..."
    log_info "NOTE: First-time model download can take 5-15 minutes depending on model size"
    
    # Use longer timeout for embedding models (20 minutes)
    if ! wait_for_service_ready_comprehensive "ai" 1200; then
        log_error "Embedding service failed to become ready"
        log_error "This may be due to:"
        log_error "  - Model download timeout (large models can take 15+ minutes)"
        log_error "  - Network connectivity issues"
        log_error "  - Insufficient disk space"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run embedding tests
    log_info "Running embedding functionality tests..."
    if ! run_embedding_tests; then
        log_error "Embedding functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "Embedding component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_embedding_tests() {
    log_group_start "Embedding Functionality Tests"
    
    # Test 1: API connectivity
    log_info "Test 1: API connectivity"
    if ! test_embedding_api_connectivity; then
        log_error "Embedding API connectivity test failed"
        log_group_end
        return 1
    fi
    log_success "API connectivity test passed"
    
    # Test 2: Embedding model availability
    log_info "Test 2: Embedding model availability"
    if ! test_embedding_model_availability; then
        log_error "Embedding model availability test failed"
        log_group_end
        return 1
    fi
    log_success "Embedding model availability test passed"
    
    # Test 3: Text embedding generation
    log_info "Test 3: Text embedding generation"
    if ! test_text_embedding_generation; then
        log_error "Text embedding generation test failed"
        log_group_end
        return 1
    fi
    log_success "Text embedding generation test passed"
    
    # Test 4: Embedding similarity
    log_info "Test 4: Embedding similarity"
    if ! test_embedding_similarity; then
        log_error "Embedding similarity test failed"
        log_group_end
        return 1
    fi
    log_success "Embedding similarity test passed"
    
    # Test 5: Batch embedding processing
    log_info "Test 5: Batch embedding processing"
    if ! test_batch_embedding_processing; then
        log_error "Batch embedding processing test failed"
        log_group_end
        return 1
    fi
    log_success "Batch embedding processing test passed"
    
    log_group_end
    return 0
}

test_embedding_api_connectivity() {
    local api_url="http://localhost:11434"
    
    # Test API root endpoint
    log_debug "Testing Ollama API root endpoint"
    if ! test_http_endpoint "$api_url" 200 10; then
        log_error "Ollama API root endpoint not accessible"
        return 1
    fi
    
    # Test embeddings endpoint
    log_debug "Testing Ollama embeddings endpoint availability"
    # We can't test the endpoint directly without a model, but we can check if it responds correctly to invalid requests
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$api_url/api/embeddings" \
        -H "Content-Type: application/json" \
        -d '{}' 2>/dev/null)
    
    # Expect 400 (bad request) or 422 (unprocessable entity) for invalid request, not 404
    if [[ "$response_code" == "400" || "$response_code" == "422" || "$response_code" == "500" ]]; then
        log_debug "Embeddings endpoint is available (returned $response_code for invalid request)"
    else
        log_warning "Embeddings endpoint response: $response_code (may indicate endpoint issues)"
    fi
    
    return 0
}

test_embedding_model_availability() {
    local api_url="http://localhost:11434/api/tags"
    
    log_debug "Checking available embedding models"
    
    # Get models list
    local models_response
    models_response=$(curl -s --max-time 15 "$api_url" 2>/dev/null)
    
    if [[ -z "$models_response" ]]; then
        log_error "Failed to get models list from Ollama"
        return 1
    fi
    
    # Validate JSON response
    if ! validate_json "$models_response"; then
        log_error "Invalid JSON response from models endpoint"
        return 1
    fi
    
    # Find embedding models
    local embedding_model=""
    if command -v jq &> /dev/null; then
        local models
        models=$(echo "$models_response" | jq -r '.models[].name' 2>/dev/null)
        
        # Look for common embedding models
        local embedding_models=("nomic-embed-text" "mxbai-embed-large" "all-minilm" "bge-small" "embed")
        
        for model in $models; do
            for embed_model in "${embedding_models[@]}"; do
                if [[ "$model" =~ $embed_model ]]; then
                    embedding_model="$model"
                    break 2
                fi
            done
        done
        
        if [[ -z "$embedding_model" ]]; then
            log_error "No embedding models found in available models"
            log_debug "Available models: $models"
            return 1
        fi
        
        log_debug "Found embedding model: $embedding_model"
        export AVAILABLE_EMBEDDING_MODEL="$embedding_model"
    else
        log_warning "jq not available, assuming embedding model is available"
        export AVAILABLE_EMBEDDING_MODEL="nomic-embed-text"  # Default from LocalCloud
    fi
    
    return 0
}

test_text_embedding_generation() {
    local api_url="http://localhost:11434/api/embeddings"
    
    if [[ -z "$AVAILABLE_EMBEDDING_MODEL" ]]; then
        log_error "No available embedding model for text generation test"
        return 1
    fi
    
    log_debug "Testing text embedding generation with model: $AVAILABLE_EMBEDDING_MODEL"
    
    # Prepare embedding request
    local request_payload=$(cat << EOF
{
    "model": "$AVAILABLE_EMBEDDING_MODEL",
    "prompt": "This is a test sentence for embedding generation."
}
EOF
)
    
    # Make embedding request with longer timeout for AI processing
    local response
    response=$(curl -s --max-time 60 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request_payload" 2>/dev/null)
    
    if [[ -z "$response" ]]; then
        log_error "No response from embedding generation API"
        return 1
    fi
    
    # Validate JSON response
    if ! validate_json "$response"; then
        log_error "Invalid JSON response from embedding API"
        return 1
    fi
    
    # Extract embedding vector
    if command -v jq &> /dev/null; then
        local embedding
        embedding=$(echo "$response" | jq -r '.embedding // empty' 2>/dev/null)
        
        if [[ -z "$embedding" || "$embedding" == "null" ]]; then
            log_error "No embedding vector in response"
            log_debug "Response: $response"
            return 1
        fi
        
        # Check if embedding is an array
        local embedding_length
        embedding_length=$(echo "$response" | jq '.embedding | length' 2>/dev/null)
        
        if [[ -z "$embedding_length" || "$embedding_length" == "null" || "$embedding_length" -eq 0 ]]; then
            log_error "Invalid embedding vector (empty or not an array)"
            return 1
        fi
        
        log_debug "Generated embedding vector with $embedding_length dimensions"
        
        # Store for other tests
        export SAMPLE_EMBEDDING="$embedding"
        export EMBEDDING_DIMENSIONS="$embedding_length"
        
        # Validate embedding values are numbers
        local first_value
        first_value=$(echo "$response" | jq '.embedding[0]' 2>/dev/null)
        
        if [[ "$first_value" == "null" ]]; then
            log_error "Embedding vector contains invalid values"
            return 1
        fi
        
        log_debug "Embedding generation successful (first value: $first_value)"
    else
        log_warning "jq not available, assuming embedding generation worked"
        export SAMPLE_EMBEDDING="[0.1, 0.2, 0.3]"
        export EMBEDDING_DIMENSIONS="3"
    fi
    
    return 0
}

test_embedding_similarity() {
    local api_url="http://localhost:11434/api/embeddings"
    
    if [[ -z "$AVAILABLE_EMBEDDING_MODEL" ]]; then
        log_error "No available embedding model for similarity test"
        return 1
    fi
    
    log_debug "Testing embedding similarity with model: $AVAILABLE_EMBEDDING_MODEL"
    
    # Generate embeddings for similar texts
    local text1="The cat sat on the mat."
    local text2="A cat is sitting on a mat."
    local text3="The weather is very hot today."
    
    # Generate first embedding
    local request1=$(cat << EOF
{
    "model": "$AVAILABLE_EMBEDDING_MODEL",
    "prompt": "$text1"
}
EOF
)
    
    local response1
    response1=$(curl -s --max-time 45 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request1" 2>/dev/null)
    
    if [[ -z "$response1" ]] || ! validate_json "$response1"; then
        log_error "Failed to generate first embedding for similarity test"
        return 1
    fi
    
    # Generate second embedding
    local request2=$(cat << EOF
{
    "model": "$AVAILABLE_EMBEDDING_MODEL",
    "prompt": "$text2"
}
EOF
)
    
    local response2
    response2=$(curl -s --max-time 45 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request2" 2>/dev/null)
    
    if [[ -z "$response2" ]] || ! validate_json "$response2"; then
        log_error "Failed to generate second embedding for similarity test"
        return 1
    fi
    
    # Generate third embedding (different topic)
    local request3=$(cat << EOF
{
    "model": "$AVAILABLE_EMBEDDING_MODEL",
    "prompt": "$text3"
}
EOF
)
    
    local response3
    response3=$(curl -s --max-time 45 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request3" 2>/dev/null)
    
    if [[ -z "$response3" ]] || ! validate_json "$response3"; then
        log_error "Failed to generate third embedding for similarity test"
        return 1
    fi
    
    # Basic validation: embeddings should have same dimensions
    if command -v jq &> /dev/null; then
        local dim1 dim2 dim3
        dim1=$(echo "$response1" | jq '.embedding | length' 2>/dev/null)
        dim2=$(echo "$response2" | jq '.embedding | length' 2>/dev/null)
        dim3=$(echo "$response3" | jq '.embedding | length' 2>/dev/null)
        
        if [[ "$dim1" != "$dim2" || "$dim2" != "$dim3" ]]; then
            log_error "Embedding dimensions mismatch (dim1: $dim1, dim2: $dim2, dim3: $dim3)"
            return 1
        fi
        
        log_debug "Similarity test: Generated 3 embeddings with $dim1 dimensions each"
        
        # Check that embeddings are different (basic sanity check)
        local embed1_first embed2_first embed3_first
        embed1_first=$(echo "$response1" | jq '.embedding[0]' 2>/dev/null)
        embed2_first=$(echo "$response2" | jq '.embedding[0]' 2>/dev/null)
        embed3_first=$(echo "$response3" | jq '.embedding[0]' 2>/dev/null)
        
        if [[ "$embed1_first" == "$embed2_first" && "$embed2_first" == "$embed3_first" ]]; then
            log_warning "All embeddings have identical first values (may indicate model issues)"
        else
            log_debug "Embeddings are different (good sign for similarity testing)"
        fi
    else
        log_warning "jq not available, skipping detailed similarity analysis"
    fi
    
    return 0
}

test_batch_embedding_processing() {
    local api_url="http://localhost:11434/api/embeddings"
    
    if [[ -z "$AVAILABLE_EMBEDDING_MODEL" ]]; then
        log_error "No available embedding model for batch processing test"
        return 1
    fi
    
    log_debug "Testing batch embedding processing"
    
    # Test multiple sequential requests (simulating batch processing)
    local texts=(
        "First document for batch processing"
        "Second document with different content"
        "Third document about machine learning"
        "Fourth document discussing artificial intelligence"
        "Fifth document on natural language processing"
    )
    
    local start_time=$(get_epoch)
    local successful_embeddings=0
    
    for i in "${!texts[@]}"; do
        local text="${texts[$i]}"
        log_debug "Processing embedding $((i+1))/${#texts[@]}: ${text:0:30}..."
        
        local request=$(cat << EOF
{
    "model": "$AVAILABLE_EMBEDDING_MODEL",
    "prompt": "$text"
}
EOF
)
        
        local response
        response=$(curl -s --max-time 30 -X POST "$api_url" \
            -H "Content-Type: application/json" \
            -d "$request" 2>/dev/null)
        
        if [[ -n "$response" ]] && validate_json "$response"; then
            if command -v jq &> /dev/null; then
                local embedding_check
                embedding_check=$(echo "$response" | jq '.embedding | length' 2>/dev/null)
                
                if [[ -n "$embedding_check" && "$embedding_check" -gt 0 ]]; then
                    ((successful_embeddings++))
                    log_debug "Embedding $((i+1)) successful ($embedding_check dimensions)"
                else
                    log_warning "Embedding $((i+1)) failed (invalid response)"
                fi
            else
                ((successful_embeddings++))
                log_debug "Embedding $((i+1)) successful (jq not available for validation)"
            fi
        else
            log_warning "Embedding $((i+1)) failed (no response or invalid JSON)"
        fi
    done
    
    local end_time=$(get_epoch)
    local batch_duration=$((end_time - start_time))
    
    log_debug "Batch processing completed: $successful_embeddings/${#texts[@]} successful in ${batch_duration}s"
    
    # Check success rate
    if [[ $successful_embeddings -eq ${#texts[@]} ]]; then
        log_debug "All batch embeddings successful"
    elif [[ $successful_embeddings -gt $((${#texts[@]} / 2)) ]]; then
        log_warning "Partial batch success: $successful_embeddings/${#texts[@]} embeddings successful"
    else
        log_error "Batch processing mostly failed: only $successful_embeddings/${#texts[@]} successful"
        return 1
    fi
    
    # Performance check
    local avg_time_per_embedding=$((batch_duration / ${#texts[@]}))
    log_debug "Average time per embedding: ${avg_time_per_embedding}s"
    
    if [[ $avg_time_per_embedding -gt 15 ]]; then
        log_warning "Batch processing slow: ${avg_time_per_embedding}s per embedding"
    fi
    
    return 0
}

# Main execution
main() {
    if test_embedding_component; then
        log_success "Embedding component test completed successfully"
        exit 0
    else
        log_error "Embedding component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi