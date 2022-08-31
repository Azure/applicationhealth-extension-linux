#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: install - creates the data dir" {
    run in_container fake-waagent install
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" = *event=installed* ]]

    diff="$(container_diff)"
    echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/apphealth"* ]]
}

@test "handler command: enable - default" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '' ''

    run start_container
    echo "$output"

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
}

@test "handler command: enable twice, process exits cleanly" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && rm /var/lib/waagent/Extension/status/0.status && fake-waagent enable && wait-for-enable status"
    push_settings '' ''

    run start_container
    echo "$output"
    [[ "$output" = *'applicationhealth-extension process terminated'* ]]

    healthy_count="$(echo "$output" | grep -c 'Health state changed to healthy')"
    echo "Enable count=$healthy_count"
    [ "$healthy_count" -eq 2 ]

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable - validates json schema" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '{"badElement":null}' ''
   
    run start_container
    echo "$output"
    [[ "$output" == *"json validation error: invalid public settings JSON: badElement"* ]]
}

@test "handler command: enable - failed tcp probe" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "tcp",
        "port": 3387
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - failed http probe" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "http",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - failed https probe" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "https",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - healthy tcp probe" {
    mk_container sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "tcp",
        "port": 8080
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable - healthy http probe" {
    mk_container sh -c "webserver -args=2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable - https unknown after 10 seconds" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && sleep 10 && rm /var/lib/waagent/Extension/status/0.status && wait-for-enable status"
    push_settings '
    {
        "protocol": "https",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus error "Application health found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - unknown http probe - no response body" {
    mk_container sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "port": 8080,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - unknown http probe - no response body - prefixing requestPath with a slash" {
    mk_container sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "port": 8080,
        "requestPath": "/health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - unknown https probe - no response body" {
    mk_container sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - numofprobes with states = unk,unk" {
    mk_container sh -c "webserver -args=3,4 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 3,
        "intervalInSeconds": 7
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 21s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk" {
    mk_container sh -c "webserver -args=2h,2h,4,4 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 5
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 10s'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is initializing"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus error "Application health found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk,h" {
    mk_container sh -c "webserver -args=2h,2h,4,4,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 8
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 16s'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 8 8 8 8)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is initializing"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
        "Health state changed to healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus error "Application health found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk,h,h" {
    mk_container sh -c "webserver -args=2h,2h,4,4,2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 5
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 10s'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 5 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"
   
    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is initializing"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
        "Health state changed to healthy"
        "Committed health state is healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus success "Application health found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: uninstall - deletes the data dir" {
    run in_container sh -c \
        "fake-waagent install && fake-waagent uninstall"
    echo "$output"
    [ "$status" -eq 0 ]

    diff="$(container_diff)" && echo "$diff"
    [[ "$diff" != */var/lib/waagent/run-command* ]]
}
