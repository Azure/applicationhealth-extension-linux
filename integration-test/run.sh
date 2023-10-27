#!/bin/bash
source integration-test/test/test_helper.bash
create_certificate
# 1_basic.bats Integration Test must be run sequentially
sudo bats integration-test/test/1_basic.bats --pretty -T --trace
sudo bats $(find integration-test/test/ -type f -name "*.bats" ! -name "1_basic.bats") --jobs 10 --pretty -T --trace
delete_certificate