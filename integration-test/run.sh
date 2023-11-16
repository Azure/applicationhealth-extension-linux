#!/bin/bash
source integration-test/test/test_helper.bash
create_certificate
# Run Sequential Integration Tests
sudo bats integration-test/test/sequential -T --trace
err1=$?
# Run Parallel Integration Tests
sudo bats integration-test/test/parallel  --jobs 10 -T --trace
err2=$?
delete_certificate
exit $((err1 + err2))