#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - vm watch disabled - vmwatch settings omitted" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'VMWatch is disabled'* ]]

    status_file="$(container_read_extension_status)"
    [[ ! $status_file == *'VMWatch'* ]]
}

@test "handler command: enable - vm watch disabled - empty vmwatch settings" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 2"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {}
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'VMWatch is disabled'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" VMWatch warning "VMWatch is disabled"
}

@test "handler command: enable - vm watch disabled - explicitly disable" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": false
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'VMWatch is disabled'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" VMWatch warning "VMWatch is disabled"
}

@test "handler command: enable - vm watch enabled - default vmwatch settings" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'VMWatch process started'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *'--input-filter disk_io:outbound_connectivity:clockskew:az_storage_blob'* ]]
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - can override default settings" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
            "tests": [
                "disk_io", 
                "outbound_connectivity"
            ],
            "parameterOverrides": {
                "ABC": "abc",
                "BCD": "bcd"
            }
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'VMWatch process started'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *'--input-filter disk_io:outbound_connectivity'* ]]
    [[ "$output" == *'Env: [ABC=abc BCD=bcd SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - app health works as expected" {
    mk_container sh -c "webserver -args=2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
            "tests": ["disk_io", "outbound_connectivity"]
        }
    }' ''
    run start_container

    echo "$output"
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is initializing"
        "Committed health state is healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'VMWatch process started'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *'--input-filter disk_io:outbound_connectivity'* ]]
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch failed - force kill vmwatch process 3 times" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 2 && pkill -f vmwatch_linux_amd64 && sleep 2 && pkill -f vmwatch_linux_amd64 && sleep 2 && pkill -f vmwatch_linux_amd64 && sleep 7"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true
        }
    }' ''
    run start_container

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "$output"
    echo "$status_file"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process started'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: signal: terminated.*"
}

@test "handler command: enable/disable - vm watch killed when disable is called" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent disable"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$output" == *'Invoking: ./Extension/bin/applicationhealth-shim disable'* ]]
    [[ "$output" == *'applicationhealth-extension process terminated'* ]]
    [[ "$output" == *'vmwatch_linux_amd64 process terminated'* ]]

    status_file="$(container_read_extension_status)"
    verify_status_item "$status_file" Disable success "Disable succeeded"
}

@test "handler command: enable/uninstall - vm watch killed when uninstall is called" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent uninstall"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$output" == *'Invoking: ./Extension/bin/applicationhealth-shim uninstall'* ]]
    [[ "$output" == *'applicationhealth-extension process terminated'* ]]
    [[ "$output" == *'vmwatch_linux_amd64 process terminated'* ]]
    [[ "$output" == *'operation=uninstall seq=0 path=/var/lib/waagent/apphealth event=uninstalled'* ]]
}
