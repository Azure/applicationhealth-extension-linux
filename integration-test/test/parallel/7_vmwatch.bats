#!/usr/bin/env bats

setup(){
    load "../test_helper"
    _load_bats_libs
    build_docker_image
    container_name="vmwatch_$BATS_TEST_NUMBER"
    extension_version=$(get_extension_version)
    echo "extension version: $extension_version"
}

teardown(){
    rm -rf "$certs_dir"
    cleanup
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
    mk_container $container_name sh -c "webserver & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *"--apphealth-version $extension_version"* ]]
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - can override default settings" {
    mk_container $container_name sh -c "webserver & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit"
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
            "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns"]
            },
            "parameterOverrides": {
                "ABC": "abc",
                "BCD": "bcd"
            }
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *'--disabled-signals clockskew:az_storage_blob:process:dns'* ]]
    [[ "$output" == *"--apphealth-version $extension_version"* ]]
    [[ "$output" == *'Env: [ABC=abc BCD=bcd SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - app health works as expected" {
    mk_container $container_name sh -c "webserver -args=2h,2h & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit"
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
            "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns"]
            }
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *'--disabled-signals clockskew:az_storage_blob:process:dns'* ]]
    [[ "$output" == *"--apphealth-version $extension_version"* ]]
    [[ "$output" == *'--memory-limit-bytes 80000000'* ]]
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch enabled - with disabled and enabled tests works as expected" {
    mk_container $container_name sh -c "webserver -args=2h,2h & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit"
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
            "signalFilters": {
                "enabledTags" : [ "Network" ],
                "disabledTags" : [ "Accuracy" ],
                "disabledSignals" : [ "outbound_connectivity", "disk_io" ],
                "enabledOptionalSignals" : [ "simple" ]
            },
            "environmentAttributes" : {
                "OutboundConnectivityEnabled" : true
            }
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'--config /var/lib/waagent/Extension/bin/VMWatch/vmwatch.conf'* ]]
    [[ "$output" == *'--disabled-signals outbound_connectivity:disk_io'* ]]
    [[ "$output" == *'--enabled-tags Network'* ]]
    [[ "$output" == *'--disabled-tags Accuracy'* ]]
    [[ "$output" == *'--enabled-optional-signals simple'* ]]
    [[ "$output" == *'--env-attributes OutboundConnectivityEnabled=true'* ]]
    [[ "$output" == *"--apphealth-version $extension_version"* ]]
    [[ "$output" == *'Env: [SIGNAL_FOLDER=/var/log/azure/Extension/events VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Extension/VE.RS.ION/vmwatch.log]'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}

@test "handler command: enable - vm watch failed - force kill vmwatch process 3 times" {
    mk_container $container_name sh -c "fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && pkill -f vmwatch_linux_amd64 && sleep 10 && pkill -f vmwatch_linux_amd64 && sleep 10 && pkill -f vmwatch_linux_amd64 && sleep 10"
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
    [[ "$output" == *'Attempt 1: Started VMWatch'* ]]
    [[ "$output" == *'Attempt 3: Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: .*"
}

@test "handler command: enable - vm watch process exit - give up after 3 restarts" {
    mk_container $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 30"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
            "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns", "outbound_connectivity", "disk_io"],
                "enabledOptionalSignals": ["test"]
            },
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
    [[ "$output" == *'Attempt 1: Started VMWatch'* ]]
    [[ "$output" == *'Attempt 3: Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: exit status 1.*"
}

@test "handler command: enable - vm watch process does not start when cgroup assignment fails" {
    mk_container $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 30"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
            "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns", "outbound_connectivity", "disk_io"],
                "enabledOptionalSignals": ["test"]
            }
        }
    }' ''
    run start_container

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    hanlder_log="$(container_read_handler_log)"
    echo "$handler_log"
    echo "$output"
    echo "$status_file"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Killing VMWatch process as cgroup assignment failed'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* VMWatch process exited. Error:.* Failed to assign VMWatch process to cgroup.**"
}

@test "handler command: enable/disable - vm watch killed when disable is called" {
    mk_container $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent disable"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$output" == *'Invoking: /var/lib/waagent/Extension/bin/applicationhealth-shim disable'* ]]
    [[ "$output" == *'applicationhealth-extension process terminated'* ]]

    status_file="$(container_read_extension_status)"
    verify_status_item "$status_file" Disable success "Disable succeeded"
}

@test "handler command: enable/uninstall - vm watch killed when uninstall is called" {
    mk_container $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent uninstall"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$output" == *'Invoking: /var/lib/waagent/Extension/bin/applicationhealth-shim uninstall'* ]]
    [[ "$output" == *'applicationhealth-extension process terminated'* ]]
    any_regex_pattern="[[:digit:]|[:space:]|[:alpha:]|[:punct:]]"
    assert_line --regexp "operation=uninstall seq=0 path=/var/lib/waagent/apphealth ${any_regex_pattern}* event=\"Handler successfully uninstalled\""
}

@test "handler command: enable - Graceful Shutdown - vm watch killed when Apphealth is killed gracefully with SIGTERM" {
    mk_container $container_name bash -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 2 && source /var/lib/waagent/test_helper.bash;kill_apphealth_extension_gracefully SIGTERM & sleep 2"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "/",
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$output" == *'event="Received shutdown request"'* ]]
    [[ "$output" == *'Successfully killed VMWatch process with PID'* ]]
    [[ "$output" == *'Application health process terminated'* ]]
}

@test "handler command: enable - Graceful Shutdown - vm watch killed when Apphealth is killed gracefully with SIGINT" {
    mk_container $container_name bash -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 2 && source /var/lib/waagent/test_helper.bash;kill_apphealth_extension_gracefully SIGINT & sleep 2"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "/",
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
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$output" == *'event="Received shutdown request"'* ]]
    [[ "$output" == *'Successfully killed VMWatch process with PID'* ]]
    [[ "$output" == *'Application health process terminated'* ]]
}

@test "handler command: enable - Forced Shutdown - vm watch killed when Apphealth is killed gracefully with SIGKILL" {
    mk_container $container_name bash -c "nc -l localhost 22 -k & export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && source /var/lib/waagent/extension-test-helpers.sh;force_kill_apphealth"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true
        }
    }' ''
    run start_container
    
    echo "$output"
    shutdown_log="$(container_read_file /var/log/azure/Extension/force-kill-extension.txt)"
    echo "$shutdown_log"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    [[ "$shutdown_log" == *'Successfully killed the apphealth extension'* ]]
    [[ "$shutdown_log" == *'Successfully killed the VMWatch extension'* ]]
}

@test "handler command: enable/uninstall - vm passes memory to commandline" {
    mk_container $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 5 && fake-waagent uninstall"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
            "memoryLimitInBytes" : 40000000
        }
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'--memory-limit-bytes 40000000'* ]]
}

# bats test_tags=linuxhostonly
@test "handler command: enable - vm watch oom - process should be killed" {
    mk_container_priviliged $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 300"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
             "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns", "outbound_connectivity", "disk_io"],
                "enabledOptionalSignals": ["test"]
            },
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
    [[ "$output" == *'Attempt 1: Started VMWatch'* ]]
    [[ "$output" == *'Attempt 3: Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]
    [[ "$output" == *'Attempt 1: VMWatch process exited'* ]]
    [[ "$output" == *'Attempt 3: VMWatch process exited'* ]]
    [[ "$output" == *'VMWatch reached max 3 retries, sleeping for 3 hours before trying again'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch error "VMWatch failed: .* Attempt 3: .* Error: signal: killed.*"
}

# bats test_tags=linuxhostonly
@test "handler command: enable - vm watch cpu - process should not use more than 1 percent cpu" {
    mk_container_priviliged $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && /var/lib/waagent/check-avg-cpu.sh vmwatch_linux 0.5 1.5"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
             "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns", "outbound_connectivity", "disk_io"],
                "enabledOptionalSignals": ["test"]
            },
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
    [[ "$output" == *'Attempt 1: Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

# bats test_tags=linuxhostonly
@test "handler command: enable - vm watch cpu - process should use more than 30 percent cpu when non-privileged" {
    mk_container $container_name sh -c "nc -l localhost 22 -k & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit && sleep 10 && /var/lib/waagent/check-avg-cpu.sh vmwatch_linux 30 150"
    push_settings '
    {
        "protocol": "tcp",
        "requestPath": "",
        "port": 22,
        "numberOfProbes": 1,
        "intervalInSeconds": 5,
        "gracePeriod": 600,
        "vmWatchSettings": {
            "enabled": true,
             "signalFilters": {
                "disabledSignals": ["clockskew", "az_storage_blob", "process", "dns", "outbound_connectivity", "disk_io"],
                "enabledOptionalSignals": ["test"]
            },
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
    [[ "$output" == *'Attempt 1: Started VMWatch'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable - vm watch enabled - Override OperationId with VMWatchCohortId " {
    mk_container $container_name sh -c "webserver -args=2h,2h & fake-waagent install && export RUNNING_IN_DEV_CONTAINER=1 && export ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE=1 && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "vmWatchSettings": {
            "enabled": true,
            "environmentAttributes" : {
                "SomeOtherKey" : "SomeOtherValue",
                "VMWatchCohortId" : "450affae-1b71-474d-885f-1598051038a0"
            }
        }
    }' ''
    run start_container

    echo "$output"
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    [[ "$output" == *'Overriding OperationId with 450affae-1b71-474d-885f-1598051038a0'* ]]
    [[ "$output" == *'Setup VMWatch command: /var/lib/waagent/Extension/bin/VMWatch/vmwatch_linux_amd64'* ]]
    [[ "$output" == *'Started VMWatch'* ]]
    [[ "$output" == *'--env-attributes SomeOtherKey=SomeOtherValue:VMWatchCohortId=450affae-1b71-474d-885f-1598051038a0'* ]]
    [[ "$output" == *'VMWatch is running'* ]]

    status_file="$(container_read_extension_status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
    verify_substatus_item "$status_file" VMWatch success "VMWatch is running"
}