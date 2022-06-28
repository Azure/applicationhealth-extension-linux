#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - rich states - interpret status code when app health state is missing" {
    mk_container sh -c "webserver -states=2m,2m,3,3,2b,2b,4,4,2u,2u & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 10 5 5 5 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
        "Health state changed to busy"
        "Committed health state is busy"
        "Health state changed to unknown"
        "Committed health state is unknown"
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - rich states - invalid app health state results in unknown" {
    mk_container sh -c "webserver -states=2h,2h,2i,2h,2i,2i & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 10 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Health state changed to healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - rich states - basic states = m,h,h,b,b,u,u,i,i" {
    mk_container sh -c "webserver -states=2m,2h,2h,2b,2b,2u,2u,2i,2i & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 15 5 5 5 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"
   
    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to busy"
        "Committed health state is busy"
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"
    
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - rich states - alternating states=unk,h,h,i,b,u,b,i,b" {
    mk_container sh -c "webserver -states=2i,2h,2h,2i,2b,2u,2b,2i,2b & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 5 5 5 5 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"
    
    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is unknown"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Health state changed to busy"
        "Health state changed to unhealthy"
        "Health state changed to busy"
        "Health state changed to unknown"
        "Health state changed to busy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - rich states - endpoint timeout results in unknown" {
    mk_container sh -c "webserver -states=2h,2t,2t & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"
    
    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

