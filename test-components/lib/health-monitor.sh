#!/bin/bash

# Health monitoring and service readiness functions for LocalCloud components

# Service readiness waiting with intelligent monitoring
wait_for_service_ready() {
    local service_name="$1"
    local timeout="${2:-300}"  # 5 minutes default
    local check_interval="${3:-5}"  # Check every 5 seconds
    
    log_info "Waiting for $service_name to be ready (timeout: ${timeout}s)..."
    
    local start_time=$(date +%s)
    local attempts=0
    local last_progress_update=0
    
    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        # Check timeout
        if [ $elapsed -gt $timeout ]; then
            log_error "$service_name failed to start within ${timeout}s"
            return 1
        fi
        
        attempts=$((attempts + 1))
        log_debug "Health check attempt $attempts for $service_name (elapsed: ${elapsed}s)"
        
        # Show progress updates for AI services (which take longer)
        if [[ "$service_name" =~ ^(ai|llm|embedding|ollama)$ ]]; then
            local progress_interval=30  # Show progress every 30 seconds for AI services
            if [ $((elapsed - last_progress_update)) -ge $progress_interval ]; then
                log_info "Still waiting for $service_name... (${elapsed}s elapsed, may take up to ${timeout}s for model download)"
                last_progress_update=$elapsed
            fi
        fi
        
        # Check service health based on type
        if check_service_health "$service_name"; then
            log_success "$service_name is ready (took ${elapsed}s)"
            return 0
        fi
        
        # Wait before next check
        sleep $check_interval
    done
}

# Service-specific health checks
check_service_health() {
    local service_name="$1"
    
    case "$service_name" in
        "ai"|"llm"|"embedding"|"ollama")
            check_ai_service_health
            ;;
        "postgres"|"database")
            check_postgres_health
            ;;
        "mongodb")
            check_mongodb_health
            ;;
        "redis-cache"|"cache")
            check_redis_health "6379"
            ;;
        "redis-queue"|"queue")
            check_redis_health "6380"
            ;;
        "minio"|"storage")
            check_minio_health
            ;;
        *)
            log_warning "Unknown service type: $service_name, using generic check"
            check_generic_service_health "$service_name"
            ;;
    esac
}

# AI service health check (Ollama)
check_ai_service_health() {
    local port="11434"
    local host="localhost"
    
    # Check if port is open
    if ! wait_for_port "$host" "$port" 10; then
        log_debug "AI service port $port not available"
        return 1
    fi
    
    # Check Ollama API endpoint with longer timeout for model loading
    if test_http_endpoint "http://$host:$port/api/tags" 200 15; then
        log_debug "AI service API is responding"
        
        # If verbose mode, show model status
        if [[ "$VERBOSE_MODE" == "true" ]]; then
            local models_info
            models_info=$(curl -s --max-time 10 "http://$host:$port/api/tags" 2>/dev/null)
            if [[ -n "$models_info" ]] && command -v jq &>/dev/null; then
                local model_count
                model_count=$(echo "$models_info" | jq '.models | length' 2>/dev/null)
                log_debug "Available models: $model_count"
                
                # Show model names if available
                if [[ "$model_count" != "null" && "$model_count" != "0" ]]; then
                    local model_names
                    model_names=$(echo "$models_info" | jq -r '.models[].name' 2>/dev/null | head -3)
                    log_debug "Models: $(echo "$model_names" | tr '\n' ', ' | sed 's/,$//')"
                fi
            fi
        fi
        
        return 0
    else
        log_debug "AI service API not responding (may still be loading models)"
        
        # Check if container is running and healthy
        if docker ps --filter "name=localcloud-" --filter "status=running" | grep -q ollama; then
            log_debug "Ollama container is running, but API not ready yet"
            
            # Show container logs in verbose mode to indicate activity
            if [[ "$VERBOSE_MODE" == "true" ]]; then
                log_debug "Checking Ollama container activity..."
                local container_logs
                container_logs=$(docker logs --tail 3 localcloud-ollama 2>/dev/null | grep -v "^$" | tail -1)
                if [[ -n "$container_logs" ]]; then
                    log_debug "Container activity: $(echo "$container_logs" | cut -c1-80)..."
                fi
            fi
        fi
        
        return 1
    fi
}

# PostgreSQL health check
check_postgres_health() {
    local port="5432"
    local host="localhost"
    
    # Check if port is open
    if ! wait_for_port "$host" "$port" 5; then
        log_debug "PostgreSQL port $port not available"
        return 1
    fi
    
    # Check PostgreSQL connection
    if test_postgres_connection "$host" "$port"; then
        log_debug "PostgreSQL connection successful"
        return 0
    else
        log_debug "PostgreSQL connection failed"
        return 1
    fi
}

# MongoDB health check
check_mongodb_health() {
    local port="27017"
    local host="localhost"
    
    # Check if port is open
    if ! wait_for_port "$host" "$port" 5; then
        log_debug "MongoDB port $port not available"
        return 1
    fi
    
    # Check MongoDB connection
    if test_mongodb_connection "$host" "$port"; then
        log_debug "MongoDB connection successful"
        return 0
    else
        log_debug "MongoDB connection failed"
        return 1
    fi
}

# Redis health check
check_redis_health() {
    local port="$1"
    local host="localhost"
    
    # Check if port is open with longer timeout for Redis startup
    if ! wait_for_port "$host" "$port" 15; then
        log_debug "Redis port $port not available"
        return 1
    fi
    
    # Additional wait for Redis to fully initialize
    sleep 2
    
    # Check Redis connection with PING command
    if test_redis_connection "$host" "$port"; then
        log_debug "Redis connection successful"
        return 0
    elif command -v redis-cli &>/dev/null; then
        # Direct Redis PING test
        if redis-cli -h "$host" -p "$port" ping 2>/dev/null | grep -q "PONG"; then
            log_debug "Redis PING successful"
            return 0
        else
            log_debug "Redis PING failed"
            return 1
        fi
    else
        log_debug "Redis connection test failed and redis-cli not available"
        return 1
    fi
}

# MinIO health check
check_minio_health() {
    local api_port="9000"
    local console_port="9001"
    local host="localhost"
    
    # Check if API port is open
    if ! wait_for_port "$host" "$api_port" 5; then
        log_debug "MinIO API port $api_port not available"
        return 1
    fi
    
    # Check MinIO health endpoint
    if test_http_endpoint "http://$host:$api_port/minio/health/live" 200 5; then
        log_debug "MinIO health check successful"
        return 0
    else
        log_debug "MinIO health check failed"
        return 1
    fi
}

# Generic service health check using LocalCloud CLI
check_generic_service_health() {
    local service_name="$1"
    
    # Try to get service status from LocalCloud
    if command -v lc &> /dev/null; then
        local service_info
        service_info=$(lc info --json 2>/dev/null)
        
        if [[ -n "$service_info" ]]; then
            # Parse JSON to check service status
            local status
            status=$(echo "$service_info" | jq -r ".services.${service_name}.status // empty" 2>/dev/null)
            
            if [[ "$status" == "healthy" || "$status" == "running" ]]; then
                log_debug "Service $service_name status: $status"
                return 0
            else
                log_debug "Service $service_name status: $status"
                return 1
            fi
        fi
    fi
    
    # Fallback: check if any containers are running
    local containers
    containers=$(docker ps --format "table {{.Names}}" | grep -i "$service_name" 2>/dev/null)
    
    if [[ -n "$containers" ]]; then
        log_debug "Found running containers for $service_name"
        return 0
    else
        log_debug "No running containers found for $service_name"
        return 1
    fi
}

# Advanced health monitoring with retry logic
wait_for_service_ready_advanced() {
    local service_name="$1"
    local timeout="${2:-300}"
    local max_retries="${3:-3}"
    
    local retry_count=0
    
    while [ $retry_count -lt $max_retries ]; do
        log_info "Health check attempt $((retry_count + 1))/$max_retries for $service_name"
        
        if wait_for_service_ready "$service_name" "$timeout"; then
            return 0
        fi
        
        retry_count=$((retry_count + 1))
        
        if [ $retry_count -lt $max_retries ]; then
            log_warning "Retrying health check for $service_name in 10 seconds..."
            sleep 10
        fi
    done
    
    log_error "Service $service_name failed to become ready after $max_retries attempts"
    return 1
}

# Container health monitoring
monitor_container_health() {
    local container_name="$1"
    local timeout="${2:-60}"
    
    log_debug "Monitoring container health: $container_name"
    
    local start_time=$(date +%s)
    
    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ $elapsed -gt $timeout ]; then
            log_error "Container $container_name health check timeout"
            return 1
        fi
        
        # Check container status
        local status
        status=$(docker inspect --format='{{.State.Health.Status}}' "$container_name" 2>/dev/null)
        
        case "$status" in
            "healthy")
                log_debug "Container $container_name is healthy"
                return 0
                ;;
            "unhealthy")
                log_error "Container $container_name is unhealthy"
                return 1
                ;;
            "starting")
                log_debug "Container $container_name is starting..."
                ;;
            "")
                # Container might not have health checks defined
                local state
                state=$(docker inspect --format='{{.State.Status}}' "$container_name" 2>/dev/null)
                if [[ "$state" == "running" ]]; then
                    log_debug "Container $container_name is running (no health check defined)"
                    return 0
                fi
                ;;
        esac
        
        sleep 5
    done
}

# Service dependency checking
check_service_dependencies() {
    local service_name="$1"
    
    case "$service_name" in
        "vector")
            # Vector depends on database
            if ! check_service_health "postgres"; then
                log_error "Vector service requires PostgreSQL to be healthy"
                return 1
            fi
            ;;
        "embedding"|"llm")
            # AI services depend on AI service
            if ! check_service_health "ai"; then
                log_error "AI services require Ollama to be healthy"
                return 1
            fi
            ;;
    esac
    
    return 0
}

# Comprehensive service readiness check
wait_for_service_ready_comprehensive() {
    local service_name="$1"
    local timeout="${2:-300}"
    
    log_group_start "Comprehensive readiness check for $service_name"
    
    # Check dependencies first
    if ! check_service_dependencies "$service_name"; then
        log_error "Service dependencies not met for $service_name"
        log_group_end
        return 1
    fi
    
    # Wait for service to be ready
    if wait_for_service_ready "$service_name" "$timeout"; then
        log_success "Service $service_name is ready and healthy"
        log_group_end
        return 0
    else
        log_error "Service $service_name failed to become ready"
        log_group_end
        return 1
    fi
}

# Export functions
export -f wait_for_service_ready check_service_health
export -f check_ai_service_health check_postgres_health check_mongodb_health
export -f check_redis_health check_minio_health check_generic_service_health
export -f wait_for_service_ready_advanced monitor_container_health
export -f check_service_dependencies wait_for_service_ready_comprehensive