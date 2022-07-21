#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - numofprobes with states = unk,unk" {
    mk_container sh -c "webserver -states=3,4 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be initializing'* ]]
}

@test "handler command: enable - numofprobes with states = unk,unk" {
    payload=$(jq -n '{
        Payload: [
            { HttpStatusCode: 200 },
            { HttpStatusCode: 300 },
            { HttpStatusCode: 400 }
        ]}'
    )
    mk_container sh -c "webserver -payload=$(echo -n $payload | jq -Rsa '.' | tr -d " \t\n\r") & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be initializing'* ]]
}