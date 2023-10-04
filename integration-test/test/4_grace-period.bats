#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
    container_name="grace-period"
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - grace period defaults even when set to 0" {
    container_name="${container_name}_1"
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 5,
        "gracePeriod": 0
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'json validation error: invalid public settings JSON: gracePeriod: Must be greater than or equal to 5'* ]]
}

@test "handler command: enable - honor grace period - http 2 probes" {
    container_name="${container_name}_2"
    mk_container $container_name sh -c "webserver -args=2i,2h,2u,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 5,
        "gracePeriod": 600
    }' ''
    run start_container

    echo "$output"

    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - honor grace period - unresponsive http probe with numberOfProbes=1" {
    container_name="${container_name}_3"
    mk_container $container_name sh -c "webserver -args=2t,2t,2t & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "/health",
        "port": 8080,
        "gracePeriod": 600
    }' ''
    run start_container

    echo "$output"

    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - bypass grace period - consecutive valid health states" {
    container_name="${container_name}_4"
    mk_container $container_name sh -c "webserver -args=2i,2h,2u,2h,2h,2,2 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriod": 600
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 10 10 10 10 10 10)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - bypass grace period - state change behavior retained" {
    container_name="${container_name}_5"
    mk_container $container_name sh -c "webserver -args=2i,2h,2u,2h,2h,2u,2u,2h,2i,2i & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriod": 600
    }' ''
    run start_container

    echo "$output"
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 10 10 10 10 10 10 10 10 10)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]
    
    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
        "Health state changed to healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - bypass grace period - larger numberOfProbes, consecutive rich states" {
    container_name="${container_name}_6"
    mk_container $container_name sh -c "webserver -args=2i,2i,2i,2i,2h,2u,2h,2u,2u,2h,2h,2h,2h,2u,2i,2i,2i,2i & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 4,
        "intervalInSeconds": 5,
        "gracePeriod": 600
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]
    
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 20 5 5 5 10 15 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - bypass / grace period expires - fail to bypass and expiration results in unknown" {
    container_name="${container_name}_7"
    mk_container $container_name sh -c "webserver -args=2u,2,2i,2i,2h,2u,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriod": 60
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 1m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]
    
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 10 30 10 10)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unhealthy"
        "Committed health state is initializing"
        "Health state changed to unknown"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - grace period expires - additional alternating health states" {
    container_name="${container_name}_8"
    mk_container $container_name sh -c "webserver -args=2i,2u,2i,2u,2i,2u,2h,2u,2h,2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriod": 60
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 1m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 10 10 10 10 10 10 0 10 10 10 10)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to unhealthy"
        "Health state changed to unknown"
        "Health state changed to unhealthy"
        "Health state changed to unknown"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is unknown"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}
