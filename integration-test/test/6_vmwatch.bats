#!/usr/bin/env bats

load test_helper

setup(){
    build_docker_image
}

teardown(){
    rm -rf "$certs_dir"
}

@test "handler command: disable - vm watch killed when disable is called" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: uninstall - vm watch killed when uninstall is called" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: enable - vm watch disabled - default settings" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: enable - vm watch disabled - configured settings" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: enable - vm watch enabled - running state with default settings" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: enable - vm watch enabled - running state with configured test settings" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: enable - vm watch enabled - running state with parameter override settings" {
    [[ "$output" == *'Not Implemented'* ]]
}

@test "handler command: enable - vm watch failed - settings invalid" {
    [[ "$output" == *'Not Implemented'* ]]
}