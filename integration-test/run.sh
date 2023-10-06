#!/bin/bash
source integration-test/test/test_helper.bash
create_certificate
# 1_basic.bats Integration Test must be run sequentially
sudo bats integration-test/test/1_basic.bats && sudo bats integration-test/test/ --jobs 10 
delete_certificate