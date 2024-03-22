#!/usr/bin/env bats

setup(){
    load "../test_helper"
    container_name="tls-config_$BATS_TEST_NUMBER"
}

teardown(){
    rm -rf "$certs_dir"
    cleanup
}

@test "handler command: enable - Testing SSLv3" {
    mk_container $container_name sh -c "webserver -args=2t,2t,2t -securityProtocol=ssl3.0 & fake-waagent install && fake-waagent enable && sleep 10 && wait-for-enable"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health",
        "port": 4430,
        "numberOfProbes": 2,
        "intervalInSeconds": 5
    }' ''
    run start_container
    echo "$output"
    # ApplicationHealth Extension should only Support from TLS 1.0 to TLS 1.3
    # if only invalid Security Protocol is supported by the application like SSLv3 
    # then the extension will receive a Standard TLS error. 
    [[ "$output" == *'Grace period set to 10s'* ]]
    [[ "$output" == *'remote error: tls: protocol version not supported'* ]]
    [[ "$output" == *'No longer honoring grace period - expired'* ]]

    enableLog="$(echo "$output" | grep 'operation=enable' | grep state)"
    
    expectedStateLogs=(
        "Health state changed to unknown"
        "Committed health state is initializing"
        "Committed health state is unknown"
    )
    verify_states "$enableLog" "${expectedStateLogs[@]}"

    status_file="$(container_read_file /var/lib/waagent/Extension/status/0.status)"
    echo $status_file
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
}


@test "handler command: enable - Testing TLS 1.0 " {
    mk_container $container_name sh -c "webserver -args=2i,2h,2h,2i,2h,2u,2h,2i,2h -securityProtocol=tls1.0 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health",
        "port": 4430,
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
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}


@test "handler command: enable - Testing TLS 1.1" {
    mk_container $container_name sh -c "webserver -args=2i,2h,2h,2i,2h,2u,2h,2i,2h -securityProtocol=tls1.1 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health",
        "port": 4430,
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
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}


@test "handler command: enable - Testing TLS 1.2" {
    mk_container $container_name sh -c "webserver -args=2i,2h,2h,2i,2h,2u,2h,2i,2h -securityProtocol=tls1.2 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health",
        "port": 4430,
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
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}

@test "handler command: enable - Testing TLS 1.3" {
    mk_container $container_name sh -c "webserver -args=2i,2h,2h,2i,2h,2u,2h,2i,2h -securityProtocol=tls1.3 & fake-waagent install && fake-waagent enable && wait-for-enable webserverexit"
    push_settings '
    {
        "protocol": "https",
        "requestPath": "health",
        "port": 4430,
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
    verify_substatus_item "$status_file" AppHealthStatus error "Application found to be unhealthy"
    verify_substatus_item "$status_file" ApplicationHealthState error Unknown
}