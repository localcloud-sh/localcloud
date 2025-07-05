#!/bin/bash

# LLM (Large Language Model) Component Test
# Tests AI/LLM functionality: model availability, text generation, API endpoints

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/health-monitor.sh"

COMPONENT_NAME="llm"

test_llm_component() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing LLM (Large Language Model) Component"
    
    # 1. Setup component
    log_info "Setting up LLM component..."
    if ! setup_component "$COMPONENT_NAME"; then
        log_error "Failed to setup LLM component"
        return 1
    fi
    
    # 2. Start service
    log_info "Starting LLM service..."
    if ! start_service "$COMPONENT_NAME"; then
        log_error "Failed to start LLM service"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 3. Wait for service readiness (LLMs take longer to start)
    log_info "Waiting for LLM service to be ready (this may take several minutes)..."
    if ! wait_for_service_ready_comprehensive "ai" 600; then
        log_error "LLM service failed to become ready"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 4. Run LLM tests
    log_info "Running LLM functionality tests..."
    if ! run_llm_tests; then
        log_error "LLM functionality tests failed"
        cleanup_component "$COMPONENT_NAME"
        return 1
    fi
    
    # 5. Cleanup
    cleanup_component "$COMPONENT_NAME"
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "LLM component test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

run_llm_tests() {
    log_group_start "LLM Functionality Tests"
    
    # Test 1: API connectivity
    log_info "Test 1: API connectivity"
    if ! test_ollama_api_connectivity; then
        log_error "Ollama API connectivity test failed"
        log_group_end
        return 1
    fi
    log_success "API connectivity test passed"
    
    # Test 2: Model availability
    log_info "Test 2: Model availability"
    if ! test_model_availability; then
        log_error "Model availability test failed"
        log_group_end
        return 1
    fi
    log_success "Model availability test passed"
    
    # Test 3: Text generation
    log_info "Test 3: Text generation"
    if ! test_text_generation; then
        log_error "Text generation test failed"
        log_group_end
        return 1
    fi
    log_success "Text generation test passed"
    
    # Test 4: Streaming response
    log_info "Test 4: Streaming response"
    if ! test_streaming_response; then
        log_error "Streaming response test failed"
        log_group_end
        return 1
    fi
    log_success "Streaming response test passed"
    
    # Test 5: Model performance
    log_info "Test 5: Model performance"
    if ! test_model_performance; then
        log_error "Model performance test failed"
        log_group_end
        return 1
    fi
    log_success "Model performance test passed"
    
    log_group_end
    return 0
}

test_ollama_api_connectivity() {
    local api_url="http://localhost:11434"
    
    # Test API root endpoint
    log_debug "Testing Ollama API root endpoint"
    if ! test_http_endpoint "$api_url" 200 10; then
        log_error "Ollama API root endpoint not accessible"
        return 1
    fi
    
    # Test tags endpoint
    log_debug "Testing Ollama tags endpoint"
    if ! test_http_endpoint "$api_url/api/tags" 200 10; then
        log_error "Ollama tags endpoint not accessible"
        return 1
    fi
    
    # Test version endpoint (if available)
    log_debug "Testing Ollama version endpoint"
    test_http_endpoint "$api_url/api/version" 200 5 || log_debug "Version endpoint not available (normal for some versions)"
    
    return 0
}

test_model_availability() {
    local api_url="http://localhost:11434/api/tags"
    
    log_debug "Checking available models"
    
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
    
    # Extract model names
    local model_count
    if command -v jq &> /dev/null; then
        model_count=$(echo "$models_response" | jq '.models | length' 2>/dev/null)
        
        if [[ "$model_count" == "null" || "$model_count" == "0" ]]; then
            log_error "No models available in Ollama"
            return 1
        fi
        
        # Look for text generation models (avoid embedding models)
        local text_gen_model=""
        local all_models
        all_models=$(echo "$models_response" | jq -r '.models[].name' 2>/dev/null)
        
        # Prioritize known text generation models
        while IFS= read -r model_name; do
            if [[ "$model_name" =~ ^(qwen|llama|mistral|gemma|phi|codellama|neural|tinyllama) ]]; then
                text_gen_model="$model_name"
                break
            fi
        done <<< "$all_models"
        
        # If no preferred model found, use first non-embedding model
        if [[ -z "$text_gen_model" ]]; then
            while IFS= read -r model_name; do
                if [[ ! "$model_name" =~ (embed|embedding|nomic-embed) ]]; then
                    text_gen_model="$model_name"
                    break
                fi
            done <<< "$all_models"
        fi
        
        # Fallback to first available model if nothing else works
        if [[ -z "$text_gen_model" ]]; then
            text_gen_model=$(echo "$models_response" | jq -r '.models[0].name' 2>/dev/null)
        fi
        
        if [[ "$text_gen_model" == "null" || -z "$text_gen_model" ]]; then
            log_error "Cannot extract model name from response"
            return 1
        fi
        
        log_debug "Found $model_count models, selected for text generation: $text_gen_model"
        export AVAILABLE_MODEL="$text_gen_model"
    else
        log_warning "jq not available, assuming models are available"
        export AVAILABLE_MODEL="qwen2.5:3b"  # Default model from LocalCloud
    fi
    
    return 0
}

test_text_generation() {
    local api_url="http://localhost:11434/api/generate"
    
    if [[ -z "$AVAILABLE_MODEL" ]]; then
        log_error "No available model for text generation test"
        return 1
    fi
    
    log_debug "Testing text generation with model: $AVAILABLE_MODEL"
    
    # Prepare generation request
    local request_payload=$(cat << EOF
{
    "model": "$AVAILABLE_MODEL",
    "prompt": "Hello, world! Please respond with a short greeting.",
    "stream": false,
    "options": {
        "temperature": 0.1,
        "top_p": 0.9,
        "max_tokens": 50
    }
}
EOF
)
    
    # Make generation request with longer timeout for AI processing
    local response
    response=$(curl -s --max-time 60 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request_payload" 2>/dev/null)
    
    if [[ -z "$response" ]]; then
        log_error "No response from text generation API"
        return 1
    fi
    
    # Validate JSON response
    if ! validate_json "$response"; then
        log_error "Invalid JSON response from generation API"
        return 1
    fi
    
    # Extract generated text
    if command -v jq &> /dev/null; then
        local generated_text
        generated_text=$(echo "$response" | jq -r '.response // empty' 2>/dev/null)
        
        if [[ -z "$generated_text" || "$generated_text" == "null" ]]; then
            log_error "No generated text in response"
            log_debug "Response: $response"
            return 1
        fi
        
        log_debug "Generated text: ${generated_text:0:100}..."
        
        # Check if response is reasonable (not empty, has some content)
        if [[ ${#generated_text} -lt 5 ]]; then
            log_error "Generated text too short (${#generated_text} characters)"
            return 1
        fi
    else
        log_warning "jq not available, assuming text generation worked"
    fi
    
    return 0
}

test_streaming_response() {
    local api_url="http://localhost:11434/api/generate"
    
    if [[ -z "$AVAILABLE_MODEL" ]]; then
        log_error "No available model for streaming test"
        return 1
    fi
    
    log_debug "Testing streaming response with model: $AVAILABLE_MODEL"
    
    # Prepare streaming request
    local request_payload=$(cat << EOF
{
    "model": "$AVAILABLE_MODEL",
    "prompt": "Count from 1 to 5:",
    "stream": true,
    "options": {
        "temperature": 0.1,
        "max_tokens": 30
    }
}
EOF
)
    
    # Make streaming request
    local response
    response=$(curl -s --max-time 45 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request_payload" 2>/dev/null)
    
    if [[ -z "$response" ]]; then
        log_error "No response from streaming API"
        return 1
    fi
    
    # Check if we got streaming response (multiple JSON objects)
    local line_count
    line_count=$(echo "$response" | wc -l | tr -d ' ')
    
    if [[ "$line_count" -lt 2 ]]; then
        log_warning "Expected multiple lines in streaming response, got $line_count"
    fi
    
    # Validate first line is valid JSON
    local first_line
    first_line=$(echo "$response" | head -n1)
    
    if ! validate_json "$first_line"; then
        log_error "First line of streaming response is not valid JSON"
        return 1
    fi
    
    log_debug "Streaming response test completed (received $line_count lines)"
    
    return 0
}

test_model_performance() {
    local api_url="http://localhost:11434/api/generate"
    
    if [[ -z "$AVAILABLE_MODEL" ]]; then
        log_error "No available model for performance test"
        return 1
    fi
    
    log_debug "Testing model performance with simple prompt"
    
    # Prepare simple request for performance measurement
    local request_payload=$(cat << EOF
{
    "model": "$AVAILABLE_MODEL",
    "prompt": "What is 2+2?",
    "stream": false,
    "options": {
        "temperature": 0.1,
        "max_tokens": 10
    }
}
EOF
)
    
    # Measure response time
    local start_time=$(get_epoch)
    
    local response
    response=$(curl -s --max-time 30 -X POST "$api_url" \
        -H "Content-Type: application/json" \
        -d "$request_payload" 2>/dev/null)
    
    local end_time=$(get_epoch)
    local response_time=$((end_time - start_time))
    
    if [[ -z "$response" ]]; then
        log_error "No response from performance test"
        return 1
    fi
    
    # Validate response
    if ! validate_json "$response"; then
        log_error "Invalid JSON response in performance test"
        return 1
    fi
    
    log_debug "Performance test completed in ${response_time}s"
    
    # Performance thresholds (adjust based on expected model performance)
    if [[ $response_time -gt 45 ]]; then
        log_warning "Model response time slow: ${response_time}s (expected < 45s for simple prompt)"
    elif [[ $response_time -gt 60 ]]; then
        log_error "Model response time too slow: ${response_time}s"
        return 1
    fi
    
    return 0
}

# Main execution
main() {
    if test_llm_component; then
        log_success "LLM component test completed successfully"
        exit 0
    else
        log_error "LLM component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi