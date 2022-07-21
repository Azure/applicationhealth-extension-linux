#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - healthy tcp probe" {
    mk_container sh -c "webserver & fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "tcp",
        "port": 443
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk,h" {
    payload=$(format_json_as_cmd_arg '{
        Payload: [
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 400 },
            { HttpStatusCode: 400 },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } }
        ]
    }')
    mk_container sh -c "webserver -payload='$payload' & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk,h,h" {
    payload=$(format_json_as_cmd_arg '{
        Payload: [
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 400 },
            { HttpStatusCode: 400 },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } }
        ]
    }')
    mk_container sh -c "webserver -payload='$payload' & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - rich states - alternating states=i,h,h,i,h,u,h,i,h" {
    payload=$(format_json_as_cmd_arg '{
        Payload: [
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Invalid" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Invalid" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Unhealthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Invalid" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } }
        ]
    }')
    mk_container sh -c "webserver -payload='$payload' & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 5 5 10 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"
    
    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Health state changed to healthy"
        "Committed health state is unknown"
        "Health state changed to healthy"
        "Health state changed to unhealthy"
        "Health state changed to healthy"
        "Health state changed to unknown"
        "Health state changed to healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}