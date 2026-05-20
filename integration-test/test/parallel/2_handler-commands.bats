#!/usr/bin/env bats

setup(){
    load "../test_helper"
    build_docker_image
    container_name="handler-command_$BATS_TEST_NUMBER"
}

teardown(){
    rm -rf "$certs_dir"
    cleanup
}

@test "handler command: install - creates the data dir" {
    mk_container $container_name sh -c "fake-waagent install && sleep 2"
    push_settings '' ''

    run start_container
    echo "$output"
    [ "$status" -eq 0 ]
    [[ "$output" = *'event="Handler successfully installed"'* ]]

    diff="$(container_diff)"
    echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/apphealth"* ]]
}

@test "handler command: enable - default" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '' ''

    run start_container
    echo "$output"

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
}

@test "handler command: enable twice, command is idempotent - initial process uninterrupted, second process exits" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && fake-waagent enable && sleep 2"
    push_settings '' ''

    run start_container
    echo "$output"

    # Second enable should detect the existing healthy process and exit cleanly (idempotent)
    [[ "$output" = *'Idempotent exit: extension already running with current configuration'* ]]

    # Only one process should have reached the healthy state (the first one)
    healthy_count="$(echo "$output" | grep -c 'Health state changed to healthy')"
    echo "Enable count=$healthy_count"
    [ "$healthy_count" -eq 1 ]

    diff="$(container_diff)"; echo "$diff"
    [[ "$diff" = *"A /var/lib/waagent/Extension/status/0.status"* ]]
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable with higher sequence number - old process killed, new process takes over" {
    # First enable runs with seq 0, becomes healthy. Then we add 1.settings (seq 1)
    # and enable again. The new process (seq 1) should kill the old process (seq 0)
    # and take over execution.
    mk_container $container_name sh -c "\
        fake-waagent install && \
        fake-waagent enable && \
        wait-for-enable && \
        cp /var/lib/waagent/Extension/config/0.settings /var/lib/waagent/Extension/config/1.settings && \
        fake-waagent enable && \
        until grep -q success /var/lib/waagent/Extension/status/1.status 2>/dev/null; do sleep 0.5; done"
    push_settings '' ''

    run start_container
    echo "$output"

    # New process should kill old process from previous sequence number
    [[ "$output" = *'Killing existing processes from previous sequence number'* ]]

    # Should NOT get an idempotent exit (different sequence numbers)
    idempotent_count="$(echo "$output" | grep -c 'Idempotent exit' || true)"
    echo "Idempotent exit count=$idempotent_count"
    [ "$idempotent_count" -eq 0 ]

    # Both processes should have reached healthy state (old with seq 0, new with seq 1)
    healthy_count="$(echo "$output" | grep -c 'Health state changed to healthy')"
    echo "Healthy count=$healthy_count"
    [ "$healthy_count" -eq 2 ]

    # Verify status file exists for the new sequence number
    status_file="$(container_read_file /var/lib/waagent/Extension/status/1.status)"
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable with lower sequence number - rejected, existing process uninterrupted" {
    # First enable runs with seq 1, becomes healthy. Then we remove 1.settings
    # (leaving only 0.settings, seq 0) and enable again. The new process (seq 0)
    # should exit immediately because mrSeqNum (1) > seqNum (0).
    mk_container $container_name sh -c "\
        fake-waagent install && \
        fake-waagent enable && \
        until grep -q success /var/lib/waagent/Extension/status/1.status 2>/dev/null; do sleep 0.5; done && \
        rm /var/lib/waagent/Extension/config/1.settings && \
        fake-waagent enable && \
        sleep 2"

    # Push settings as both 0.settings and 1.settings so first enable uses seq 1
    push_settings '' ''
    cfg_file="$(mktemp)"
    docker cp "$TEST_CONTAINER:/var/lib/waagent/Extension/config/0.settings" "$cfg_file"
    docker cp "$cfg_file" "$TEST_CONTAINER:/var/lib/waagent/Extension/config/1.settings"
    rm -f "$cfg_file"

    run start_container
    echo "$output"

    # Second enable (seq 0) should fail because mrSeqNum (1) > seqNum (0)
    [[ "$output" = *'most recent sequence number 1 is greater than the requested sequence number 0'* ]]

    # First process should have reached healthy state
    healthy_count="$(echo "$output" | grep -c 'Health state changed to healthy')"
    echo "Healthy count=$healthy_count"
    [ "$healthy_count" -ge 1 ]
}

@test "handler command: enable with lower sequence number - rejected even when existing process is unhealthy" {
    # Same as above but validates sequence number takes precedence regardless of
    # existing process health. Even if the existing process were unhealthy, a lower
    # sequence number should still be rejected.
    mk_container $container_name sh -c "\
        fake-waagent install && \
        fake-waagent enable && \
        until grep -q success /var/lib/waagent/Extension/status/1.status 2>/dev/null; do sleep 0.5; done && \
        rm /var/lib/waagent/Extension/config/1.settings && \
        fake-waagent enable && \
        sleep 2"

    push_settings '' ''
    cfg_file="$(mktemp)"
    docker cp "$TEST_CONTAINER:/var/lib/waagent/Extension/config/0.settings" "$cfg_file"
    docker cp "$cfg_file" "$TEST_CONTAINER:/var/lib/waagent/Extension/config/1.settings"
    rm -f "$cfg_file"

    run start_container
    echo "$output"

    # Second enable (seq 0) should still fail - sequence number takes precedence over healthiness
    [[ "$output" = *'most recent sequence number 1 is greater than the requested sequence number 0'* ]]

    # Should NOT get an idempotent exit (different sequence numbers)
    idempotent_count="$(echo "$output" | grep -c 'Idempotent exit' || true)"
    echo "Idempotent exit count=$idempotent_count"
    [ "$idempotent_count" -eq 0 ]
}

@test "handler command: enable - validates json schema" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '{"badElement":null}' ''
   
    run start_container
    echo "$output"
    [[ "$output" == *"json validation error: invalid public settings JSON: badElement"* ]]
}

@test "handler command: enable - failed tcp probe" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "tcp",
        "port": 3387
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - failed http probe" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "http",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - failed https probe" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "https",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - healthy tcp probe" {
    mk_container $container_name sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable"
    push_settings '
    {
        "protocol": "tcp",
        "port": 8080
    }' ''
    run start_container
    echo "$output"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable - healthy http probe" {
    mk_container $container_name sh -c "webserver -args=2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "requestPath": "health",
        "port": 8080
    }' ''
    run start_container

    echo "$output"
    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'No longer honoring grace period - successful probes'* ]]

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: enable - https unknown after 10 seconds" {
    mk_container $container_name sh -c "fake-waagent install && fake-waagent enable && wait-for-enable && sleep 10 && rm /var/lib/waagent/Extension/status/0.status && wait-for-enable status"
    push_settings '
    {
        "protocol": "https",
        "port": 88,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedTimeDifferences=(0 5)
    verify_state_change_timestamps "$enableLog" "${expectedTimeDifferences[@]}"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - unknown http probe - no response body" {
    mk_container $container_name sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "port": 8080,
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"
    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - unknown http probe - no response body - prefixing requestPath with a slash" {
    mk_container $container_name sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "http",
        "port": 8080,
        "requestPath": "/health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - unknown https probe - no response body" {
    mk_container $container_name sh -c "webserver_shim && fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health"
    }' ''
    run start_container
    echo "$output"

    [[ "$output" == *'Grace period set to 5s'* ]]
    [[ "$output" == *'Honoring grace period'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"

    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - numofprobes with states = unk,unk" {
    mk_container $container_name sh -c "webserver -args=3,4 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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
    
    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState transitioning Initializing
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk" {
    mk_container $container_name sh -c "webserver -args=2h,2h,4,4 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    expectedTimeDifferences=(0 5 5 5)
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

    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk,h" {
    mk_container $container_name sh -c "webserver -args=2h,2h,4,4,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - numofprobes with states = h,h,unk,unk,h,h" {
    mk_container $container_name sh -c "webserver -args=2h,2h,4,4,2h,2h & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
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

    verify_substatus_item "$status_file" AppHealthStatus success "Application found to be healthy"
    verify_substatus_item "$status_file" ApplicationHealthState success Healthy
}

@test "handler command: uninstall - deletes the data dir" {
    run in_container sh -c \
        "fake-waagent install && fake-waagent uninstall"
    echo "$output"
    [ "$status" -eq 0 ]

    diff="$(container_diff)" && echo "$diff"
    [[ "$diff" != */var/lib/waagent/run-command* ]]
}
