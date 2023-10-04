#!/bin/bash
source integration-test/test/test_helper.bash
create_certificate
sudo bats integration-test/test --jobs 10
delete_certificate