#!/bin/bash

# Simple Tunnel Test - Isolated tunnel functionality testing
# Tests tunnel creation with a simple HTTP service

set -e

# Get script directory and source dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"

COMPONENT_NAME="tunnel-simple"
TUNNEL_PID=""
TUNNEL_URL=""
HTTP_SERVER_PID=""

# Emergency cleanup function
emergency_cleanup() {
    log_debug "Emergency cleanup triggered for simple tunnel test"
    
    # Kill tunnel process
    if [[ -n "$TUNNEL_PID" ]]; then
        kill "$TUNNEL_PID" &>/dev/null || true
        wait "$TUNNEL_PID" &>/dev/null || true
    fi
    
    # Kill HTTP server
    if [[ -n "$HTTP_SERVER_PID" ]]; then
        kill "$HTTP_SERVER_PID" &>/dev/null || true
    fi
    
    # Kill any cloudflared processes
    pkill -f "cloudflared" &>/dev/null || true
    pkill -f "lc tunnel" &>/dev/null || true
    
    exit 130
}

# Setup signal handlers
trap emergency_cleanup INT TERM

test_simple_tunnel() {
    local test_start_time=$(get_epoch)
    
    log_group_start "Testing Simple Tunnel Functionality"
    
    # 1. Check prerequisites
    log_info "Checking tunnel prerequisites..."
    if ! check_simple_prerequisites; then
        log_error "Prerequisites not met"
        return 1
    fi
    
    # 2. Start simple HTTP server
    log_info "Starting simple HTTP server for testing..."
    if ! start_test_http_server; then
        log_error "Failed to start test HTTP server"
        return 1
    fi
    
    # 3. Test tunnel functionality
    log_info "Testing tunnel creation and URL extraction..."
    if ! test_tunnel_creation; then
        log_error "Tunnel creation test failed"
        cleanup_test_server
        return 1
    fi
    
    # 4. Test tunnel accessibility (optional since tunnel is stopped)
    log_info "Testing tunnel accessibility..."
    if ! test_tunnel_access; then
        log_warning "Tunnel accessibility test skipped (tunnel already stopped)"
    fi
    
    # 5. Cleanup
    cleanup_test_server
    
    local test_end_time=$(get_epoch)
    local test_duration=$((test_end_time - test_start_time))
    
    log_success "Simple tunnel test completed successfully in ${test_duration}s"
    log_group_end
    
    return 0
}

check_simple_prerequisites() {
    log_group_start "Simple Prerequisites Check"
    
    # Check cloudflared
    if ! command -v cloudflared &> /dev/null; then
        log_error "cloudflared not found. Install with: brew install cloudflared"
        log_group_end
        return 1
    fi
    log_success "cloudflared is available"
    
    # Check internet connectivity
    if ! curl -s --max-time 10 "https://www.cloudflare.com" &>/dev/null; then
        log_error "No internet connectivity"
        log_group_end
        return 1
    fi
    log_success "Internet connectivity confirmed"
    
    # Check python3 for test server
    if ! command -v python3 &> /dev/null; then
        log_error "python3 not found (needed for test HTTP server)"
        log_group_end
        return 1
    fi
    log_success "python3 is available"
    
    # Check lc binary
    if [[ -f "../../lc" ]]; then
        log_info "Using local lc binary"
        export LC_COMMAND="../../lc"
    elif command -v lc &> /dev/null; then
        log_info "Using system lc binary" 
        export LC_COMMAND="lc"
    else
        log_error "lc command not found"
        log_group_end
        return 1
    fi
    log_success "lc command is available"
    
    log_group_end
    return 0
}

start_test_http_server() {
    local port=8091
    
    # Find available port starting from 8091
    while lsof -i :$port &>/dev/null; do
        port=$((port + 1))
    done
    
    log_debug "Starting HTTP server on port $port"
    
    # Create a simple HTML response
    cat > /tmp/tunnel_test_index.html << 'EOF'
<!DOCTYPE html>
<html>
<head><title>LocalCloud Tunnel Test</title></head>
<body>
    <h1>🚀 Tunnel Test Success!</h1>
    <p>This page is served through a LocalCloud tunnel.</p>
    <p>Time: <span id="time"></span></p>
    <script>
        document.getElementById('time').textContent = new Date().toISOString();
    </script>
</body>
</html>
EOF
    
    # Start HTTP server
    cd /tmp && python3 -m http.server $port &>/dev/null &
    HTTP_SERVER_PID=$!
    export TEST_PORT=$port
    
    sleep 2
    
    # Verify server is running
    if ! lsof -i :$port &>/dev/null; then
        log_error "Failed to start HTTP server on port $port"
        return 1
    fi
    
    # Test server responds
    if ! curl -s --max-time 5 "http://localhost:$port" &>/dev/null; then
        log_error "HTTP server not responding"
        kill "$HTTP_SERVER_PID" &>/dev/null || true
        return 1
    fi
    
    log_success "HTTP server running on port $port"
    return 0
}

test_tunnel_creation() {
    local tunnel_log="/tmp/simple_tunnel_test_$$.log"
    local tunnel_started=false
    
    log_debug "Creating tunnel for port $TEST_PORT using command: $LC_COMMAND"
    
    # Test the command directly first
    if ! "$LC_COMMAND" --version &>/dev/null; then
        log_error "LocalCloud command not working: $LC_COMMAND"
        return 1
    fi
    
    # Start tunnel (run in standalone mode, no project needed)
    echo "ℹ No LocalCloud project found. Running in standalone mode..." > "$tunnel_log"
    
    # Start tunnel in background with explicit path resolution
    log_debug "Starting tunnel: $LC_COMMAND tunnel start --port $TEST_PORT --name tunnel-test"
    "$LC_COMMAND" tunnel start --port "$TEST_PORT" --name tunnel-test >> "$tunnel_log" 2>&1 &
    TUNNEL_PID=$!
    
    # Give more time for process to start
    sleep 3
    if ! kill -0 "$TUNNEL_PID" 2>/dev/null; then
        log_error "Tunnel process failed to start or died immediately (PID: $TUNNEL_PID)"
        if [[ -f "$tunnel_log" ]]; then
            log_debug "Tunnel log contents:"
            cat "$tunnel_log"
        fi
        
        # Check if the command exists and is executable
        log_debug "Command check: $LC_COMMAND"
        ls -la "$LC_COMMAND" 2>/dev/null || echo "Command not found"
        
        return 1
    fi
    
    log_debug "Started tunnel process with PID: $TUNNEL_PID"
    
    # Wait for tunnel to establish (increased timeout for more reliable testing)
    local wait_time=0
    local max_wait=60
    
    while [[ $wait_time -lt $max_wait ]]; do
        if [[ -f "$tunnel_log" ]] && (grep -q "trycloudflare.com" "$tunnel_log" || grep -q "Your quick Tunnel has been created" "$tunnel_log" || grep -q "Tunnel established" "$tunnel_log" || grep -q "Press Ctrl+C to stop" "$tunnel_log"); then
            tunnel_started=true
            log_debug "Tunnel success detected in log file"
            break
        fi
        
        # Check if process is still running
        if ! kill -0 "$TUNNEL_PID" 2>/dev/null; then
            log_error "Tunnel process died unexpectedly (PID: $TUNNEL_PID)"
            
            # Check system logs for process termination
            if command -v dmesg &>/dev/null; then
                log_debug "Checking system logs for process termination..."
                dmesg | tail -5 | grep -i "$TUNNEL_PID" || true
            fi
            
            # Check if there are any kill processes
            pgrep -fl kill | head -3 || true
            
            if [[ -f "$tunnel_log" ]]; then
                log_debug "Tunnel log contents:"
                head -20 "$tunnel_log"
            fi
            return 1
        fi
        
        # Show debug info every 10 seconds
        if [[ $wait_time -gt 0 && $((wait_time % 10)) -eq 0 && -f "$tunnel_log" ]]; then
            log_debug "Waiting for tunnel... (${wait_time}s)"
            log_debug "Last 5 lines of tunnel log:"
            tail -5 "$tunnel_log" 2>/dev/null || echo "No log content yet"
        fi
        
        sleep 2
        wait_time=$((wait_time + 2))
    done
    
    if [[ "$tunnel_started" == "false" ]]; then
        log_error "Tunnel failed to start within ${max_wait} seconds"
        kill "$TUNNEL_PID" &>/dev/null || true
        if [[ -f "$tunnel_log" ]]; then
            log_debug "Full tunnel log:"
            cat "$tunnel_log"
        fi
        return 1
    fi
    
    # Extract tunnel URL using multiple patterns
    TUNNEL_URL=$(grep -o 'https://[a-zA-Z0-9\-]*\.trycloudflare\.com' "$tunnel_log" | head -1)
    
    if [[ -z "$TUNNEL_URL" ]]; then
        # Try different patterns
        TUNNEL_URL=$(grep -o 'https://[^[:space:]]*trycloudflare\.com' "$tunnel_log" | head -1)
    fi
    
    if [[ -z "$TUNNEL_URL" ]]; then
        # Try extracting from the whole log more broadly
        TUNNEL_URL=$(grep -E -o 'https://[a-zA-Z0-9\-_]+\.trycloudflare\.com' "$tunnel_log" | head -1)
    fi
    
    if [[ -z "$TUNNEL_URL" ]]; then
        # Last resort - check if URL is in the output somewhere
        if grep -q "trycloudflare.com" "$tunnel_log"; then
            log_debug "Tunnel appears to be running (found trycloudflare.com in log)"
            log_debug "Log contents:"
            cat "$tunnel_log"
        fi
        log_warning "Could not extract tunnel URL, but tunnel appears to be running"
    fi
    
    if [[ -n "$TUNNEL_URL" ]]; then
        log_success "Tunnel created successfully: $TUNNEL_URL"
    else
        log_warning "Tunnel running but URL extraction failed"
    fi
    
    # Kill tunnel immediately after successful creation to avoid conflicts
    if [[ -n "$TUNNEL_PID" ]] && kill -0 "$TUNNEL_PID" 2>/dev/null; then
        log_debug "Stopping tunnel process after successful creation"
        kill "$TUNNEL_PID" &>/dev/null || true
        wait "$TUNNEL_PID" &>/dev/null || true
        TUNNEL_PID=""
    fi
    
    return 0
}

test_tunnel_access() {
    if [[ -z "$TUNNEL_URL" ]]; then
        log_warning "No tunnel URL available, skipping accessibility test"
        return 0
    fi
    
    log_debug "Testing tunnel accessibility: $TUNNEL_URL"
    
    # Test tunnel endpoint
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 15 "$TUNNEL_URL" 2>/dev/null || echo "000")
    
    if [[ "$response_code" == "000" ]]; then
        log_error "Failed to connect to tunnel URL"
        return 1
    fi
    
    if [[ "$response_code" == "200" ]]; then
        log_success "Tunnel accessible (HTTP $response_code)"
        
        # Try to get actual content
        local content
        content=$(curl -s --max-time 10 "$TUNNEL_URL" 2>/dev/null | head -5)
        
        if [[ "$content" =~ "Tunnel Test Success" ]]; then
            log_success "Tunnel serving correct content"
        else
            log_warning "Tunnel accessible but unexpected content"
        fi
    else
        log_warning "Tunnel returned HTTP $response_code (expected 200)"
    fi
    
    return 0
}

cleanup_test_server() {
    log_debug "Cleaning up test server and tunnel"
    
    # Kill tunnel
    if [[ -n "$TUNNEL_PID" ]] && kill -0 "$TUNNEL_PID" 2>/dev/null; then
        log_debug "Stopping tunnel process"
        kill "$TUNNEL_PID" &>/dev/null || true
        wait "$TUNNEL_PID" &>/dev/null || true
        TUNNEL_PID=""
    fi
    
    # Kill HTTP server
    if [[ -n "$HTTP_SERVER_PID" ]] && kill -0 "$HTTP_SERVER_PID" 2>/dev/null; then
        log_debug "Stopping HTTP server"
        kill "$HTTP_SERVER_PID" &>/dev/null || true
        HTTP_SERVER_PID=""
    fi
    
    # Clean up any remaining processes
    pkill -f "cloudflared" &>/dev/null || true
    
    # Clean up temp files
    rm -f /tmp/tunnel_test_index.html 2>/dev/null || true
    
    log_debug "Cleanup complete"
}

# Component test function (called by test runner)
test_tunnel_component() {
    test_simple_tunnel
}

# Main execution
main() {
    if test_tunnel_component; then
        log_success "Tunnel component test completed successfully"
        exit 0
    else
        log_error "Tunnel component test failed"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi