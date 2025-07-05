#!/bin/bash

# Common utility functions for LocalCloud component testing

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# GitHub Actions detection
is_github_actions() {
    [[ "$GITHUB_ACTIONS" == "true" ]]
}

# Logging functions
log_info() {
    local message="$1"
    if is_github_actions; then
        echo "::notice::$message"
    else
        echo -e "${BLUE}[INFO]${NC} $message"
    fi
}

log_success() {
    local message="$1"
    if is_github_actions; then
        echo "::notice::✓ $message"
    else
        echo -e "${GREEN}[SUCCESS]${NC} ✓ $message"
    fi
}

log_warning() {
    local message="$1"
    if is_github_actions; then
        echo "::warning::$message"
    else
        echo -e "${YELLOW}[WARNING]${NC} ⚠ $message"
    fi
}

log_error() {
    local message="$1"
    if is_github_actions; then
        echo "::error::$message"
    else
        echo -e "${RED}[ERROR]${NC} ✗ $message"
    fi >&2
}

log_debug() {
    local message="$1"
    if [[ "$VERBOSE_MODE" == "true" ]]; then
        if is_github_actions; then
            echo "::debug::$message"
        else
            echo -e "${PURPLE}[DEBUG]${NC} $message"
        fi
    fi
}

# GitHub Actions group functions
log_group_start() {
    local group_name="$1"
    if is_github_actions; then
        echo "::group::$group_name"
    else
        echo -e "${CYAN}=== $group_name ===${NC}"
    fi
}

log_group_end() {
    if is_github_actions; then
        echo "::endgroup::"
    fi
}

# Resource monitoring
monitor_resources() {
    if is_github_actions; then
        log_info "Memory usage: $(free -h 2>/dev/null | awk '/^Mem:/ {print $3 "/" $2}' || echo 'N/A')"
        log_info "Disk usage: $(df -h / 2>/dev/null | awk 'NR==2 {print $3 "/" $2}' || echo 'N/A')"
    fi
}

# Component management functions

# Ensure clean component setup by removing all existing components
ensure_clean_component_setup() {
    log_debug "Checking for existing components to remove..."
    
    # Get list of currently enabled components
    local enabled_components
    enabled_components=$(lc component list 2>/dev/null | grep "Enabled" | awk '{print $1}' || echo "")
    
    if [[ -n "$enabled_components" ]]; then
        log_debug "Found existing components: $enabled_components"
        
        # Remove each enabled component
        echo "$enabled_components" | while read -r comp; do
            if [[ -n "$comp" ]]; then
                log_debug "Removing existing component: $comp"
                echo "y" | lc component remove "$comp" &>/dev/null || true
            fi
        done
        
        # Wait a moment for configuration to update
        sleep 1
        
        log_debug "Existing components removed for clean test isolation"
    else
        log_debug "No existing components found"
    fi
}

setup_component() {
    local component="$1"
    
    log_group_start "Setting up component: $component"
    
    # Ensure clean component setup by removing any existing components first
    log_info "Ensuring clean component isolation..."
    ensure_clean_component_setup
    
    # Check if component is already enabled
    local component_status
    component_status=$(lc component list 2>/dev/null | grep "^$component" || echo "")
    
    if [[ "$component_status" =~ "Enabled" ]]; then
        log_info "Component $component is already enabled"
        log_group_end
        return 0
    fi
    
    # Add component
    log_info "Adding component: $component"
    
    # Try different approaches to add the component
    local add_success=false
    
    # Approach 1: Try with --non-interactive flag
    if lc component add "$component" --non-interactive &>/dev/null; then
        add_success=true
    # Approach 2: Try without --non-interactive flag
    elif lc component add "$component" &>/dev/null; then
        add_success=true
    # Approach 3: Try with automated yes responses
    elif echo -e "y\ny\ny\ny\ny" | lc component add "$component" &>/dev/null; then
        add_success=true
    # Approach 4: Try with expect if available
    elif command -v expect &>/dev/null; then
        expect -c "
            spawn lc component add $component
            expect {
                \"*?*\" { send \"y\r\"; exp_continue }
                eof
            }
        " &>/dev/null && add_success=true
    fi
    
    if [[ "$add_success" == "true" ]]; then
        log_success "Component $component added successfully"
        
        # Wait a moment for the configuration to be updated
        sleep 2
        
        # Verify the component was added and is the ONLY enabled component
        component_status=$(lc component list 2>/dev/null | grep "^$component" || echo "")
        if [[ "$component_status" =~ "Enabled" ]]; then
            log_debug "Component $component verified as enabled"
            
            # Verify test isolation - check if any other components are enabled
            local other_enabled
            other_enabled=$(lc component list 2>/dev/null | grep "Enabled" | grep -v "^$component" || echo "")
            
            if [[ -n "$other_enabled" ]]; then
                log_warning "Test isolation issue: Other components are also enabled:"
                echo "$other_enabled" | while read -r line; do
                    log_warning "  $line"
                done
                log_warning "This may cause the test to start unexpected services"
            else
                log_debug "Test isolation confirmed: Only $component is enabled"
            fi
        else
            log_warning "Component $component may not be properly enabled"
            log_debug "Component status: $component_status"
            
            # Try to show the full component list for debugging
            if [[ "$VERBOSE_MODE" == "true" ]]; then
                log_debug "Full component list:"
                lc component list 2>&1 | head -20 | while read -r line; do
                    log_debug "  $line"
                done
            fi
        fi
    else
        log_error "Failed to add component: $component"
        log_debug "This may be due to missing dependencies or configuration issues"
        log_group_end
        return 1
    fi
    
    log_group_end
    return 0
}

cleanup_component() {
    local component="$1"
    
    if [[ "$CLEANUP_ON_ERROR" == "false" ]]; then
        log_info "Skipping cleanup for component: $component (--no-cleanup specified)"
        return 0
    fi
    
    log_group_start "Cleaning up component: $component"
    
    # Stop services first
    log_info "Stopping services..."
    lc stop &>/dev/null || true
    
    # Force cleanup any leftover containers
    log_info "Cleaning up Docker containers..."
    cleanup_docker_containers
    
    # Remove component
    log_info "Removing component: $component"
    if echo "y" | lc component remove "$component" &>/dev/null; then
        log_success "Component $component removed successfully"
    else
        log_warning "Failed to remove component: $component"
    fi
    
    log_group_end
    return 0
}

# Force cleanup of LocalCloud Docker containers
cleanup_docker_containers() {
    log_debug "Cleaning up LocalCloud Docker containers..."
    
    # Stop and remove all LocalCloud containers
    local containers
    containers=$(docker ps -a --filter "name=localcloud-" --format "{{.Names}}" 2>/dev/null || echo "")
    
    if [[ -n "$containers" ]]; then
        log_debug "Found containers to cleanup: $containers"
        
        # Stop containers
        echo "$containers" | xargs -r docker stop &>/dev/null || true
        
        # Remove containers
        echo "$containers" | xargs -r docker rm &>/dev/null || true
        
        log_debug "Docker containers cleaned up"
    else
        log_debug "No LocalCloud containers found"
    fi
    
    # Clean up any orphaned networks
    local networks
    networks=$(docker network ls --filter "name=localcloud_" --format "{{.Name}}" 2>/dev/null || echo "")
    
    if [[ -n "$networks" ]]; then
        log_debug "Cleaning up LocalCloud networks: $networks"
        echo "$networks" | xargs -r docker network rm &>/dev/null || true
    fi
}

start_service() {
    local component="$1"
    
    log_group_start "Starting service for component: $component"
    
    # Start all services (LocalCloud will start only enabled components)
    log_info "Starting LocalCloud services..."
    
    # Try different start approaches
    local start_success=false
    
    # Approach 1: Try with --detach flag
    if lc start --detach &>/dev/null; then
        start_success=true
    # Approach 2: Try without --detach flag
    elif lc start &>/dev/null; then
        start_success=true
    # Approach 3: Try with explicit component
    elif lc start "$component" &>/dev/null; then
        start_success=true
    fi
    
    if [[ "$start_success" == "true" ]]; then
        log_success "Services started successfully"
        
        # Give services a moment to initialize
        sleep 3
        
        # Verify services are running
        if lc ps &>/dev/null; then
            log_debug "Services verified as running"
        else
            log_warning "Service verification failed"
        fi
    else
        log_error "Failed to start services"
        log_debug "This may be due to Docker issues or missing dependencies"
        
        # Try to get more debug information
        if [[ "$VERBOSE_MODE" == "true" ]]; then
            log_debug "Attempting to get error details..."
            lc start 2>&1 | head -10 | while read -r line; do
                log_debug "Start error: $line"
            done
        fi
        
        log_group_end
        return 1
    fi
    
    log_group_end
    return 0
}

stop_service() {
    local component="$1"
    
    log_group_start "Stopping service for component: $component"
    
    log_info "Stopping LocalCloud services..."
    if lc stop &>/dev/null; then
        log_success "Services stopped successfully"
    else
        log_warning "Failed to stop services cleanly"
    fi
    
    log_group_end
    return 0
}

# Service URL helpers
get_service_url() {
    local service_name="$1"
    
    # Try to get service URL from LocalCloud
    if command -v lc &> /dev/null; then
        lc service url "$service_name" 2>/dev/null || echo ""
    else
        echo ""
    fi
}

# Generic service port helpers
get_service_port() {
    local service_name="$1"
    
    case "$service_name" in
        "postgres"|"database")
            echo "5432"
            ;;
        "mongodb")
            echo "27017"
            ;;
        "redis-cache"|"cache")
            echo "6379"
            ;;
        "redis-queue"|"queue")
            echo "6380"
            ;;
        "minio"|"storage")
            echo "9000"
            ;;
        "ai"|"ollama"|"llm"|"embedding")
            echo "11434"
            ;;
        *)
            echo ""
            ;;
    esac
}

# HTTP test helpers
test_http_endpoint() {
    local url="$1"
    local expected_code="${2:-200}"
    local timeout="${3:-10}"
    
    log_debug "Testing HTTP endpoint: $url (expecting $expected_code)"
    
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$timeout" "$url" 2>/dev/null)
    
    if [[ "$response_code" == "$expected_code" ]]; then
        log_debug "HTTP test passed: $url returned $response_code"
        return 0
    else
        log_debug "HTTP test failed: $url returned $response_code (expected $expected_code)"
        return 1
    fi
}

# JSON test helpers
test_json_response() {
    local url="$1"
    local jq_filter="${2:-.}"
    local timeout="${3:-10}"
    
    log_debug "Testing JSON response: $url with filter '$jq_filter'"
    
    local response
    response=$(curl -s --max-time "$timeout" "$url" 2>/dev/null)
    
    if [[ -z "$response" ]]; then
        log_debug "JSON test failed: empty response from $url"
        return 1
    fi
    
    if echo "$response" | jq -e "$jq_filter" >/dev/null 2>&1; then
        log_debug "JSON test passed: $url returned valid JSON"
        return 0
    else
        log_debug "JSON test failed: $url returned invalid JSON or filter failed"
        return 1
    fi
}

# Database test helpers
test_postgres_connection() {
    local host="${1:-localhost}"
    local port="${2:-5432}"
    local database="${3:-localcloud}"
    local username="${4:-localcloud}"
    local password="${5:-localcloud}"
    
    log_debug "Testing PostgreSQL connection: $username@$host:$port/$database"
    
    # Test connection using psql
    if command -v psql &> /dev/null; then
        PGPASSWORD="$password" psql -h "$host" -p "$port" -U "$username" -d "$database" -c "SELECT 1;" &>/dev/null
        return $?
    else
        log_debug "psql not available, skipping PostgreSQL connection test"
        return 0
    fi
}

test_mongodb_connection() {
    local host="${1:-localhost}"
    local port="${2:-27017}"
    local database="${3:-localcloud}"
    local username="${4:-localcloud}"
    local password="${5:-localcloud}"
    
    log_debug "Testing MongoDB connection: $username@$host:$port/$database"
    
    # Test connection using mongosh
    if command -v mongosh &> /dev/null; then
        mongosh "mongodb://$username:$password@$host:$port/$database?authSource=admin" --eval "db.runCommand({ping: 1})" &>/dev/null
        return $?
    else
        log_debug "mongosh not available, skipping MongoDB connection test"
        return 0
    fi
}

test_redis_connection() {
    local host="${1:-localhost}"
    local port="${2:-6379}"
    
    log_debug "Testing Redis connection: $host:$port"
    
    # Test connection using redis-cli
    if command -v redis-cli &> /dev/null; then
        redis-cli -h "$host" -p "$port" ping &>/dev/null
        return $?
    else
        log_debug "redis-cli not available, skipping Redis connection test"
        return 0
    fi
}

# Utility functions
wait_for_port() {
    local host="$1"
    local port="$2"
    local timeout="${3:-30}"
    
    log_debug "Waiting for port $port on $host (timeout: ${timeout}s)"
    
    local start_time=$(date +%s)
    while true; do
        # Try multiple approaches to check port availability
        local port_check_success=false
        
        # Method 1: Use /dev/tcp (bash built-in)
        if (echo > /dev/tcp/$host/$port) 2>/dev/null; then
            port_check_success=true
        # Method 2: Use nc if available
        elif command -v nc &>/dev/null && nc -z "$host" "$port" 2>/dev/null; then
            port_check_success=true
        # Method 3: Use telnet if available
        elif command -v telnet &>/dev/null && echo "quit" | telnet "$host" "$port" 2>/dev/null | grep -q "Connected"; then
            port_check_success=true
        # Method 4: Use curl for HTTP-like services
        elif [[ "$port" =~ ^(80|443|8080|9000|9001)$ ]] && command -v curl &>/dev/null; then
            if curl -s --connect-timeout 1 "http://$host:$port" &>/dev/null; then
                port_check_success=true
            fi
        fi
        
        if [[ "$port_check_success" == "true" ]]; then
            log_debug "Port $port is available on $host"
            return 0
        fi
        
        local current_time=$(date +%s)
        if [ $((current_time - start_time)) -gt $timeout ]; then
            log_debug "Timeout waiting for port $port on $host"
            return 1
        fi
        
        sleep 1
    done
}

# Time helpers
get_timestamp() {
    date +"%Y-%m-%d %H:%M:%S"
}

get_epoch() {
    date +%s
}

# Validation helpers
validate_json() {
    local json_string="$1"
    echo "$json_string" | jq . >/dev/null 2>&1
}

validate_url() {
    local url="$1"
    [[ "$url" =~ ^https?:// ]]
}

# Export functions for use in other scripts
export -f log_info log_success log_warning log_error log_debug
export -f log_group_start log_group_end
export -f monitor_resources
export -f setup_component cleanup_component start_service stop_service
export -f get_service_url get_service_port
export -f test_http_endpoint test_json_response
export -f test_postgres_connection test_mongodb_connection test_redis_connection
export -f wait_for_port get_timestamp get_epoch validate_json validate_url
export -f is_github_actions