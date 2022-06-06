#!/usr/bin/env bats

load test_helper

@test "handler command: enable - alternating numofprobes with rich health = m,h,h,x,b,x,b,x,b" {
    mk_container sh -c "webserver -states=m,h,h,x,b,x,b,x,b & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
        "State changed to unknown"
        "Committed health state is unknown"
        "State changed to healthy"
        "Committed health state is healthy"
        "State changed to unknown"
        "State changed to busy"
        "State changed to unknown"
        "State changed to busy"
        "State changed to unknown"
        "State changed to busy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}