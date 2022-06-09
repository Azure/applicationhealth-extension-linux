#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - log shows grace period not set" {
    mk_container sh -c "webserver -states=h,h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Grace period not set'* ]]
    [[ "$output" == *'Committed health state is healthy'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - http probe honor grace period" {
    mk_container sh -c "webserver -states=x,h,u,b & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 5,
        "gracePeriodInMinutes": 10
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
        "Health state changed to busy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be initializing'* ]]
}

@test "handler command: enable - unresponsive http probe with numberOfProbes=1 honors grace period" {
    mk_container sh -c "webserver -states=m,m,m & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "/health",
        "port": 8080,
        "gracePeriodInMinutes": 10
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

@test "handler command: enable - tcp probe with numberOfProbes=1 bypasses grace period" {
    mk_container sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "tcp",
        "port": 3387,
        "gracePeriodInMinutes": 10
    }' ''
    run start_container

    echo "$output"

    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - failed tcp probe honors grace period" {
    mk_container sh -c "webserver -uptime=15 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "tcp",
        "port": 3387,
        "numberOfProbes": 2,
        "gracePeriodInMinutes": 10
    }' ''
    run start_container

    echo "$output"

    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unhealthy"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be initializing'* ]]
}

@test "handler command: enable - bypassing grace period with consecutive valid health states" {
    mk_container sh -c "webserver -states=x,h,u,b,b,h,h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriodInMinutes": 10
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
        "Health state changed to busy"
        "Committed health state is busy"
        "Health state changed to healthy"
        "Committed health state is healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - state change behavior retained after bypassing grace period" {
    mk_container sh -c "webserver -states=x,h,u,b,b,h,h,u,b,x,x & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriodInMinutes": 10
    }' ''
    run start_container

    echo "$output"
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 10 10 10 10 10 10 10 10 10 10)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]
    
    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to busy"
        "Committed health state is busy"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
        "Health state changed to busy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - larger numberOfProbes, consecutive rich states pass through grace period" {
    mk_container sh -c "webserver -states=x,x,x,x,h,u,b,h,u,u,h,h,h,h,u,b,b,b,b & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 4,
        "intervalInSeconds": 5,
        "gracePeriodInMinutes": 10
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 10m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]
    
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 20 5 5 5 5 10 15 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to busy"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
        "Health state changed to busy"
        "Committed health state is busy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be busy'* ]]
}


@test "handler command: enable - unhealthy when grace period expires" {
    mk_container sh -c "webserver -states=x,x,x,x,x,x,x & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriodInMinutes": 1
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 1m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]
    
    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 60)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Committed health state is unhealthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - unhealthy when grace period expires with additional alternating health states" {
    mk_container sh -c "webserver -states=x,u,x,u,x,u,h,u,h,u & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080,
        "numberOfProbes": 2,
        "intervalInSeconds": 10,
        "gracePeriodInMinutes": 1
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 1m'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 10 10 10 10 10 10 0 10 10 10)
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
        "Committed health state is unhealthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}
