#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - valid custom metrics" {
    payload=$(format_json_as_cmd_arg '{
        Payload: [
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Unhealthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Unhealthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{\"name\":\"Frank\", \"age\":23, \"locations\":[\"hawaii\", \"los angeles\", \"bellevue\"] }" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{\"hello\": { \"hello2\": { \"hello3\": { \"hello4\": \"world\" } } } }" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Unhealthy", CustomMetrics: "{}" } },
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
    
    expectedTimeDifferences=(0 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unhealthy"
        "Committed health state is initializing"
        "Committed health state is unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}


@test "handler command: enable - invalid custom metrics result in unknown" {
    payload=$(format_json_as_cmd_arg '{
        Payload: [
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "\"Platypus\"" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "Platypus" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "\"10\"" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "10" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{hello}" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{\"hello\"}" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{10}" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "[]" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "[\"name\"]" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "[\"null\"]" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: " " } }
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
    
    expectedTimeDifferences=(0 5 5 5 60)
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}
