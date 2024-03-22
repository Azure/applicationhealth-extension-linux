setup_suite(){
    load "../test_helper"
    build_docker_image
}

teardown_suite(){
    rm_image
}