#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: install - creates the data dir" {
    run in_container fake-waagent install
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" = *event=installed* ]]

    diff="$(container_diff)"
    echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/apphealth"* ]]
}

@test "handler command: enable - default" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '' ''

    run start_container
    echo "$output"

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable twice, process exits cleanly" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && rm /var/lib/waagent/Extension/status/0.status && fake-waagent enable && wait-for-enable status"
    push_settings '' ''

    run start_container
    echo "$output"
    [[ "$output" = *'applicationhealth-extension process terminated'* ]]

    healthy_count="$(echo "$output" | grep -c 'Health state changed to healthy')"
    echo "Enable count=$healthy_count"
    [ "$healthy_count" -eq 2 ]

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - validates json schema" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '{"badElement":null}' ''
   
    run start_container
    echo "$output"
    [[ "$output" == *"json validation error: invalid public settings JSON: badElement"* ]]
}

@test "handler command: enable - failed tcp probe" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "tcp",
        "port": 3387
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - unknown http probe" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "http",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - unknown https probe" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "https",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - unknown after 10 seconds" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && sleep 10 && rm /var/lib/waagent/Extension/status/0.status && wait-for-enable status"
    push_settings '
    {
        "protocol": "https",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
}

@test "handler command: enable - healthy tcp probe" {
    mk_container sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable"
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

@test "handler command: enable - healthy http probe" {
    mk_container sh -c "webserver -states=h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "port": 8080,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - healthy http probe prefixing requestPath with a slash" {
    mk_container sh -c "webserver -states=h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "port": 8080,
        "requestPath": "/health"
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - healthy https probe" {
    mk_container sh -c "webserver -states=h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - numofprobes with states = uu" {
    mk_container sh -c "webserver -states=u,u & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - numofprobes with states = huu" {
    mk_container sh -c "webserver -states=h,u,u & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - numofprobes with states = huuh" {
    mk_container sh -c "webserver -states=h,u,u,h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedTimeDifferences=(0 8 8 8)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
        "Health state changed to healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unhealthy'* ]]
}

@test "handler command: enable - numofprobes with states = huuhh" {
    mk_container sh -c "webserver -states=h,u,u,h,h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
   
    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unhealthy"
        "Committed health state is unhealthy"
        "Health state changed to healthy"
        "Committed health state is healthy"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be healthy'* ]]
}

@test "handler command: enable - numofprobes with rich health states = m,h,h,b,b,u,u,x,x" {
    mk_container sh -c "webserver -states=m,h,h,b,b,u,u,x,x & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

@test "handler command: enable - alternating numofprobes with rich health = m,h,h,x,b,u,b,x,b" {
    mk_container sh -c "webserver -states=m,h,h,x,b,u,b,x,b & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

@test "handler command: enable - endpoint timeout results in unknown" {
    mk_container sh -c "webserver -states=h,t,t & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

@test "handler command: enable - invalid or missing app health state in response body results in unknown" {
    mk_container sh -c "webserver -states=h,m,m,h,h,x,x & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    
    expectedTimeDifferences=(0 5 5 5 5 5 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
        "Health state changed to healthy"
        "Committed health state is healthy"
        "Health state changed to unknown"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application health found to be unknown'* ]]
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

@test "handler command: enable - honor grace period" {
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

@test "handler command: enable - states pass through when grace period expires" {
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

@test "handler command: uninstall - deletes the data dir" {
    run in_container sh -c \
        "fake-waagent install && fake-waagent uninstall"
    echo "$output"
    [ "$status" -eq 0 ]

    diff="$(container_diff)" && echo "$diff"
    [[ "$diff" != */var/lib/waagent/run-command* ]]
}
