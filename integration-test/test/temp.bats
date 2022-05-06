#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - numofprobes with rich health states = i,h,h,d,d,di,di,b,b,u,u,unk,unk" {
    mk_container sh -c "webserver -states=d,u,u,di,di & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    expectedTimeDifferences=(0 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"
   
    [[ "$output" == *'Committed health state is initializing'* ]]
    [[ "$output" == *'Committed health state is healthy'* ]]
    [[ "$output" == *'Committed health state is draining'* ]]
    [[ "$output" == *'Committed health state is disabled'* ]]
    [[ "$output" == *'Committed health state is busy'* ]]
    [[ "$output" == *'Committed health state is unhealthy'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unknown'* ]]
}

@test "handler command: enable - endpoint timeout results in unknown" {
    mk_container sh -c "webserver -states=i,t,t & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Committed health state is initializing'* ]]
    [[ "$output" == *'Committed health state is unknown'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unknown'* ]]
}