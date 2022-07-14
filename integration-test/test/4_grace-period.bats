#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - grace period defaults even when set to 0" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
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
    mk_container sh -c "webserver -states=2i,2h,2u,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be initializing'* ]]
}

@test "handler command: enable - honor grace period - unresponsive http probe with numberOfProbes=1" {
    mk_container sh -c "webserver -states=2t,2t,2t & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be initializing'* ]]
}

@test "handler command: enable - bypass grace period - consecutive valid health states" {
    mk_container sh -c "webserver -states=2i,2h,2u,2h,2h,2,2 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - bypass grace period - state change behavior retained" {
    mk_container sh -c "webserver -states=2i,2h,2u,2h,2h,2u,2u,2h,2i,2i & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - bypass grace period - larger numberOfProbes, consecutive rich states" {
    mk_container sh -c "webserver -states=2i,2i,2i,2i,2h,2u,2h,2u,2u,2h,2h,2h,2h,2u,2i,2i,2i,2i & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - bypass / grace period expires - fail to bypass and expiration results in unknown" {
    mk_container sh -c "webserver -states=2u,2m,2i,2i,2h,2u,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - grace period expires - additional alternating health states" {
    mk_container sh -c "webserver -states=2i,2u,2i,2u,2i,2u,2h,2u,2h,2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}
