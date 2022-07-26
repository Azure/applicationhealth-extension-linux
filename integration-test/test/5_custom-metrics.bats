#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: enable - correctly handle malformed custom metrics json" {
    payload=$(format_string_as_cmd_arg '{
        Payload: [
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Unhealthy", CustomMetrics: "{\"rollingUpgradePolicy\":{\"upgradeAllowed\": true, \"phase\": 3 } }" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{\"rollingUpgradePolicy\":{\"upgradeAllowed\": true \"phase\": 3 } }" } },
            { HttpStatusCode: 200, ResponseBody: { ApplicationHealthState: "Healthy", CustomMetrics: "{"rollingUpgradePolicy":{"upgradeAllowed": true, "phase": 3 } }" } },
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
        "Health state changed to unhealthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be debug'* ]]
}
