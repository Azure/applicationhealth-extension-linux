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
    echo "$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
}

@test "handler command: enable twice, process exits cleanly" {
    mk_container sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && rm /var/lib/waagent/Extension/status/0.status && fake-waagent enable && wait-for-enable status"
    push_settings '' ''

    run start_container
    echo "$output"
    [[ "$output" = *'applicationhealth-extension process terminated'* ]]

    healthy_count="$(echo "$output" | grep -c 'State changed to healthy')"
    echo "Enable count=$healthy_count"
    [ "$healthy_count" -eq 2 ]

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unhealthy'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unknown'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unknown'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unknown'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
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
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unhealthy'* ]]
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

    [[ "$output" == *'Committed health state is healthy'* ]]
    [[ "$output" == *'Committed health state is unhealthy'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unhealthy'* ]]
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
    expectedTimeDifferences=(0 8 8 8 8)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    [[ "$output" == *'Committed health state is healthy'* ]]
    [[ "$output" == *'Committed health state is unhealthy'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unhealthy'* ]]
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
   
    [[ "$output" == *'Committed health state is healthy'* ]]
    [[ "$output" == *'Committed health state is unhealthy'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be healthy'* ]]
}

@test "handler command: enable - numofprobes with rich health states = i,h,h,d,d,di,di,b,b,u,u,unk,unk" {
    mk_container sh -c "webserver -states=i,h,h,d,d,di,di,b,b,u,u,unk,unk & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    expectedTimeDifferences=(0 5 5 5 5 5 5 5 5 5 5 5 5)
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

@test "handler command: enable - invalid or missing app health state in response body results in unknown" {
    mk_container sh -c "webserver -states=i,u,u,m,m,h,h,x,x & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    [[ "$output" == *'Committed health state is unhealthy'* ]]
    [[ "$output" == *'Committed health state is unknown'* ]]
    [[ "$output" == *'Committed health state is healthy'* ]]
    [[ "$output" == *'Committed health state is unknown'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo "status_file=$status_file"; [[ "$status_file" = *'Application found to be unknown'* ]]
}

@test "handler command: uninstall - deletes the data dir" {
    run in_container sh -c \
        "fake-waagent install && fake-waagent uninstall"
    echo "$output"
    [ "$status" -eq 0 ]

    diff="$(container_diff)" && echo "$diff"
    [[ "$diff" != */var/lib/waagent/run-command* ]]
}