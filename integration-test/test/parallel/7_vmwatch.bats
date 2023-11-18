#!/usr/bin/env bats

load ../test_helper

setup(){
    build_docker_image
    container_name="vmwatch_$BATS_TEST_NUMBER"
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - vm watch disabled - vmwatch settings omitted" {
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 2"
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
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - can override default settings" {
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Env: [ABC=abc BCD=bcd SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - app health works as expected" {
    mk_container $container_name sh -c "webserver -args=2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch failed - force kill vmwatch process 3 times" {
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && pkill -f vmwatch_linux_amd64 && sleep 10 && pkill -f vmwatch_linux_amd64 && sleep 10 && pkill -f vmwatch_linux_amd64 && sleep 10"
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
    hanlder_log="$(container_read_handler_log)"
    echo "$handler_log"
    vmwatch_log="$(container_read_vmwatch_log)"
    echo "$vmwatch_log"
    echo "$output"
    echo "$status_file"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process started'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: signal: terminated.*"
}

@test "handler command: enable - vm watch process exit - give up after 3 restarts" {
    mk_container $container_name sh -c "webserver & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 30"
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
            "tests": ["test"],
            "parameterOverrides": {
                "TEST_EXIT_PROCESS": "true"
            }
        }
    }' ''
    run start_container

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    hanlder_log="$(container_read_handler_log)"
    echo "$handler_log"
    vmwatch_log="$(container_read_vmwatch_log)"
    echo "$vmwatch_log"
    echo "$output"
    echo "$status_file"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process started'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: exit status 1.*"
}

@test "handler command: enable/disable - vm watch killed when disable is called" {
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent disable"
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

    [[ "$output" == *'Invoking: /var/lib/waagent/Extension/bin/applicationhealth-shim disable'* ]]
    [[ "$output" == *'applicationhealth-extension process terminated'* ]]
    [[ "$output" == *'vmwatch_linux_amd64 process terminated'* ]]

    status_file="$(container_read_extension_status)"
    verify_status_item "$status_file" Disable success "Disable succeeded"
}

@test "handler command: enable/uninstall - vm watch killed when uninstall is called" {
    mk_container $container_name sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent uninstall"
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

    [[ "$output" == *'Invoking: /var/lib/waagent/Extension/bin/applicationhealth-shim uninstall'* ]]
    [[ "$output" == *'applicationhealth-extension process terminated'* ]]
    [[ "$output" == *'vmwatch_linux_amd64 process terminated'* ]]
    [[ "$output" == *'operation=uninstall seq=0 path=/var/lib/waagent/apphealth event=uninstalled'* ]]
}

# bats test_tags=linuxhostonly
@test "handler command: enable - vm watch oom - process should be killed" {
    mk_container_priviliged $container_name sh -c "webserver & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 300"
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
            "tests": ["test"],
            "parameterOverrides": {
                "TEST_ALLOCATE_MEMORY": "true"
            }
        }
    }' ''
    run start_container

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    hanlder_log="$(container_read_handler_log)"
    echo "$handler_log"
    vmwatch_log="$(container_read_vmwatch_log)"
    echo "$vmwatch_log"
    echo "$output"
    echo "$status_file"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process started'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: signal: killed.*"
}

# bats test_tags=linuxhostonly
@test "handler command: enable - vm watch cpu - process should not use more than 1 percent cpu" {
    mk_container_priviliged $container_name sh -c "webserver & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && /var/lib/waagent/get-avg-vmwatch-cpu.sh"
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
            "tests": ["test"],
            "parameterOverrides": {
                "TEST_HIGH_CPU": "true"
            }
        }
    }' ''
    run start_container

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    hanlder_log="$(container_read_handler_log)"
    avg_cpu="$(container_read_file /var/log/azure/Extension/vmwatch-avg-cpu-check.txt)"
    echo "$handler_log"
    vmwatch_log="$(container_read_vmwatch_log)"
    echo "$vmwatch_log"
    echo "$output"
    echo "$status_file"
    echo "$avg_cpu"
    
    [[ "$avg_cpu" == *'PASS'* ]]
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

# bats test_tags=linuxhostonly
@test "handler command: enable - vm watch cpu - process should use more than 1 percent cpu when non-privileged" {
    mk_container $container_name sh -c "webserver & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && /var/lib/waagent/get-avg-vmwatch-cpu.sh"
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
            "tests": ["test"],
            "parameterOverrides": {
                "TEST_HIGH_CPU": "true"
            }
        }
    }' ''
    run start_container

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    hanlder_log="$(container_read_handler_log)"
    avg_cpu="$(container_read_file /var/log/azure/Extension/vmwatch-avg-cpu-check.txt)"
    echo "$handler_log"
    vmwatch_log="$(container_read_vmwatch_log)"
    echo "$vmwatch_log"
    echo "$output"
    echo "$status_file"
    echo "$avg_cpu"
    
    [[ "$avg_cpu" == *'FAIL'* ]]
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process started'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}
