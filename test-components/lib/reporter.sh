#!/bin/bash

# Test result reporting and output formatting

# Global variables (bash 4+ associative arrays)
if [[ ${BASH_VERSION%%.*} -ge 4 ]]; then
    declare -A TEST_RESULTS_DATA=()
    declare -A TEST_METADATA=()
else
    # Fallback for older bash versions
    TEST_RESULTS_DATA_KEYS=()
    TEST_RESULTS_DATA_VALUES=()
    TEST_METADATA_KEYS=()
    TEST_METADATA_VALUES=()
fi
REPORT_FORMAT="console"
REPORT_OUTPUT_DIR="./reports"
REPORT_TIMESTAMP=""

# Initialize reporter
init_reporter() {
    local format="$1"
    local output_dir="$2"
    
    REPORT_FORMAT="$format"
    REPORT_OUTPUT_DIR="$output_dir"
    REPORT_TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
    
    # Create output directory
    mkdir -p "$REPORT_OUTPUT_DIR"
    
    # Initialize report files based on format
    case "$format" in
        "json")
            init_json_report
            ;;
        "junit")
            init_junit_report
            ;;
        "console")
            init_console_report
            ;;
        *)
            log_warning "Unknown report format: $format, using console"
            REPORT_FORMAT="console"
            ;;
    esac
    
    log_debug "Reporter initialized with format: $REPORT_FORMAT"
}

# Initialize JSON report
init_json_report() {
    local json_file="$REPORT_OUTPUT_DIR/test-results.json"
    cat > "$json_file" << EOF
{
    "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "format": "json",
    "tests": [],
    "summary": {
        "total": 0,
        "passed": 0,
        "failed": 0,
        "duration": 0
    }
}
EOF
    log_debug "JSON report initialized: $json_file"
}

# Initialize JUnit report
init_junit_report() {
    local junit_file="$REPORT_OUTPUT_DIR/junit.xml"
    cat > "$junit_file" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
    <testsuite name="LocalCloud Component Tests" timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")">
    </testsuite>
</testsuites>
EOF
    log_debug "JUnit report initialized: $junit_file"
}

# Initialize console report
init_console_report() {
    local console_file="$REPORT_OUTPUT_DIR/console.log"
    echo "LocalCloud Component Test Report - $(date)" > "$console_file"
    echo "=======================================" >> "$console_file"
    echo "" >> "$console_file"
    log_debug "Console report initialized: $console_file"
}

# Report individual test result
report_test_result() {
    local component="$1"
    local result="$2"
    local duration="$3"
    local error_message="$4"
    
    # Store test data
    TEST_RESULTS_DATA["$component"]="$result"
    TEST_METADATA["${component}_duration"]="$duration"
    TEST_METADATA["${component}_error"]="$error_message"
    TEST_METADATA["${component}_timestamp"]="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    
    # Report based on format
    case "$REPORT_FORMAT" in
        "json")
            report_json_result "$component" "$result" "$duration" "$error_message"
            ;;
        "junit")
            report_junit_result "$component" "$result" "$duration" "$error_message"
            ;;
        "console")
            report_console_result "$component" "$result" "$duration" "$error_message"
            ;;
    esac
}

# Report JSON result
report_json_result() {
    local component="$1"
    local result="$2"
    local duration="$3"
    local error_message="$4"
    
    local json_file="$REPORT_OUTPUT_DIR/test-results.json"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    # Create test entry
    local test_entry=$(cat << EOF
{
    "component": "$component",
    "result": "$result",
    "duration": $duration,
    "timestamp": "$timestamp",
    "error": "$error_message"
}
EOF
)
    
    # Add to JSON file (requires jq for proper JSON manipulation)
    if command -v jq &> /dev/null; then
        local temp_file=$(mktemp)
        jq --argjson test "$test_entry" '.tests += [$test]' "$json_file" > "$temp_file"
        mv "$temp_file" "$json_file"
    else
        # Fallback: simple append (not valid JSON but readable)
        echo "$test_entry" >> "$REPORT_OUTPUT_DIR/test-results-raw.json"
    fi
}

# Report JUnit result
report_junit_result() {
    local component="$1"
    local result="$2"
    local duration="$3"
    local error_message="$4"
    
    local junit_file="$REPORT_OUTPUT_DIR/junit.xml"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    # Create test case entry
    local test_case=""
    if [[ "$result" == "PASS" ]]; then
        test_case="        <testcase name=\"$component\" classname=\"LocalCloud.Component\" time=\"$duration\" timestamp=\"$timestamp\" />"
    else
        test_case="        <testcase name=\"$component\" classname=\"LocalCloud.Component\" time=\"$duration\" timestamp=\"$timestamp\">
            <failure message=\"$error_message\">$error_message</failure>
        </testcase>"
    fi
    
    # Insert before closing testsuite tag
    if command -v sed &> /dev/null; then
        sed -i.bak "s|</testsuite>|$test_case\n    </testsuite>|" "$junit_file"
        rm -f "$junit_file.bak"
    else
        echo "$test_case" >> "$REPORT_OUTPUT_DIR/junit-raw.xml"
    fi
}

# Report console result
report_console_result() {
    local component="$1"
    local result="$2"
    local duration="$3"
    local error_message="$4"
    
    local console_file="$REPORT_OUTPUT_DIR/console.log"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    
    local status_symbol="✓"
    if [[ "$result" == "FAIL" ]]; then
        status_symbol="✗"
    fi
    
    local result_line="[$timestamp] $status_symbol $component: $result (${duration}s)"
    if [[ -n "$error_message" ]]; then
        result_line="$result_line - $error_message"
    fi
    
    echo "$result_line" >> "$console_file"
    
    # Also log to console
    if [[ "$result" == "PASS" ]]; then
        log_success "$component test completed in ${duration}s"
    else
        log_error "$component test failed in ${duration}s: $error_message"
    fi
}

# Finalize report with summary
finalize_report() {
    local total_tests="$1"
    local passed_tests="$2"
    local failed_tests="$3"
    local total_duration="$4"
    
    case "$REPORT_FORMAT" in
        "json")
            finalize_json_report "$total_tests" "$passed_tests" "$failed_tests" "$total_duration"
            ;;
        "junit")
            finalize_junit_report "$total_tests" "$passed_tests" "$failed_tests" "$total_duration"
            ;;
        "console")
            finalize_console_report "$total_tests" "$passed_tests" "$failed_tests" "$total_duration"
            ;;
    esac
    
    # Create summary files
    create_summary_files "$total_tests" "$passed_tests" "$failed_tests" "$total_duration"
}

# Finalize JSON report
finalize_json_report() {
    local total_tests="$1"
    local passed_tests="$2"
    local failed_tests="$3"
    local total_duration="$4"
    
    local json_file="$REPORT_OUTPUT_DIR/test-results.json"
    
    if command -v jq &> /dev/null; then
        local temp_file=$(mktemp)
        jq --arg total "$total_tests" \
           --arg passed "$passed_tests" \
           --arg failed "$failed_tests" \
           --arg duration "$total_duration" \
           '.summary.total = ($total | tonumber) |
            .summary.passed = ($passed | tonumber) |
            .summary.failed = ($failed | tonumber) |
            .summary.duration = ($duration | tonumber)' \
           "$json_file" > "$temp_file"
        mv "$temp_file" "$json_file"
        
        log_info "JSON report generated: $json_file"
    else
        log_warning "jq not available, JSON report may be incomplete"
    fi
}

# Finalize JUnit report
finalize_junit_report() {
    local total_tests="$1"
    local passed_tests="$2"
    local failed_tests="$3"
    local total_duration="$4"
    
    local junit_file="$REPORT_OUTPUT_DIR/junit.xml"
    
    # Update testsuite attributes
    if command -v sed &> /dev/null; then
        sed -i.bak "s|<testsuite name=\"LocalCloud Component Tests\"|<testsuite name=\"LocalCloud Component Tests\" tests=\"$total_tests\" failures=\"$failed_tests\" time=\"$total_duration\"|" "$junit_file"
        rm -f "$junit_file.bak"
        
        log_info "JUnit report generated: $junit_file"
    else
        log_warning "sed not available, JUnit report may be incomplete"
    fi
}

# Finalize console report
finalize_console_report() {
    local total_tests="$1"
    local passed_tests="$2"
    local failed_tests="$3"
    local total_duration="$4"
    
    local console_file="$REPORT_OUTPUT_DIR/console.log"
    
    cat >> "$console_file" << EOF

===============================
SUMMARY
===============================
Total tests:    $total_tests
Passed:         $passed_tests
Failed:         $failed_tests
Total duration: ${total_duration}s
Success rate:   $(( passed_tests * 100 / total_tests ))%

Generated at: $(date)
EOF
    
    log_info "Console report generated: $console_file"
}

# Create summary files
create_summary_files() {
    local total_tests="$1"
    local passed_tests="$2"
    local failed_tests="$3"
    local total_duration="$4"
    
    # Create simple summary file
    local summary_file="$REPORT_OUTPUT_DIR/summary.txt"
    cat > "$summary_file" << EOF
LocalCloud Component Test Summary
=================================
Date: $(date)
Total Tests: $total_tests
Passed: $passed_tests
Failed: $failed_tests
Duration: ${total_duration}s
Success Rate: $(( passed_tests * 100 / total_tests ))%
EOF
    
    # Create GitHub Actions summary if in GitHub Actions
    if is_github_actions; then
        create_github_actions_summary "$total_tests" "$passed_tests" "$failed_tests" "$total_duration"
    fi
    
    log_info "Summary files created in: $REPORT_OUTPUT_DIR"
}

# Create GitHub Actions summary
create_github_actions_summary() {
    local total_tests="$1"
    local passed_tests="$2"
    local failed_tests="$3"
    local total_duration="$4"
    
    if [[ -n "$GITHUB_STEP_SUMMARY" ]]; then
        cat >> "$GITHUB_STEP_SUMMARY" << EOF
## LocalCloud Component Test Results

| Metric | Value |
|--------|-------|
| Total Tests | $total_tests |
| Passed | $passed_tests |
| Failed | $failed_tests |
| Duration | ${total_duration}s |
| Success Rate | $(( passed_tests * 100 / total_tests ))% |

### Test Details

| Component | Result | Duration |
|-----------|--------|----------|
EOF
        
        # Add individual test results
        for component in "${!TEST_RESULTS_DATA[@]}"; do
            local result="${TEST_RESULTS_DATA[$component]}"
            local duration="${TEST_METADATA[${component}_duration]}"
            local status_emoji="✅"
            
            if [[ "$result" == "FAIL" ]]; then
                status_emoji="❌"
            fi
            
            echo "| $component | $status_emoji $result | ${duration}s |" >> "$GITHUB_STEP_SUMMARY"
        done
        
        echo "" >> "$GITHUB_STEP_SUMMARY"
        
        # Add failure details if any
        if [[ $failed_tests -gt 0 ]]; then
            echo "### Failed Tests" >> "$GITHUB_STEP_SUMMARY"
            echo "" >> "$GITHUB_STEP_SUMMARY"
            
            for component in "${!TEST_RESULTS_DATA[@]}"; do
                local result="${TEST_RESULTS_DATA[$component]}"
                local error_message="${TEST_METADATA[${component}_error]}"
                
                if [[ "$result" == "FAIL" ]]; then
                    echo "- **$component**: $error_message" >> "$GITHUB_STEP_SUMMARY"
                fi
            done
        fi
        
        log_info "GitHub Actions summary generated"
    fi
}

# Export test results for external consumption
export_test_results() {
    local export_format="$1"
    local export_file="$2"
    
    case "$export_format" in
        "csv")
            export_csv_results "$export_file"
            ;;
        "json")
            cp "$REPORT_OUTPUT_DIR/test-results.json" "$export_file" 2>/dev/null || true
            ;;
        "junit")
            cp "$REPORT_OUTPUT_DIR/junit.xml" "$export_file" 2>/dev/null || true
            ;;
        *)
            log_error "Unknown export format: $export_format"
            return 1
            ;;
    esac
}

# Export CSV results
export_csv_results() {
    local csv_file="$1"
    
    echo "Component,Result,Duration,Timestamp,Error" > "$csv_file"
    
    for component in "${!TEST_RESULTS_DATA[@]}"; do
        local result="${TEST_RESULTS_DATA[$component]}"
        local duration="${TEST_METADATA[${component}_duration]}"
        local timestamp="${TEST_METADATA[${component}_timestamp]}"
        local error="${TEST_METADATA[${component}_error]}"
        
        echo "$component,$result,$duration,$timestamp,\"$error\"" >> "$csv_file"
    done
    
    log_info "CSV export generated: $csv_file"
}

# Get test results for programmatic access
get_test_result() {
    local component="$1"
    echo "${TEST_RESULTS_DATA[$component]:-}"
}

get_test_duration() {
    local component="$1"
    echo "${TEST_METADATA[${component}_duration]:-}"
}

get_test_error() {
    local component="$1"
    echo "${TEST_METADATA[${component}_error]:-}"
}

# Export functions
export -f init_reporter report_test_result finalize_report
export -f export_test_results get_test_result get_test_duration get_test_error
export -f create_github_actions_summary