#!/bin/bash

# set the filter skip tests that can only run when the host is really a linux machine (not just WSL or docker)
FILTER=-"-filter-tags !linuxhostonly"

for i in "$@"; do
  case $i in
    --all)
      FILTER=
      shift
      ;;
    -*|--*)
      echo "Unknown option $i"
      exit 1
      ;;
    *)
      ;;
  esac
done

source integration-test/test/test_helper.bash
create_certificate
# Run Sequential Integration Tests
bats integration-test/test/sequential -T --trace 
err1=$?
# Run Parallel Integration Tests
bats integration-test/test/parallel  --jobs 10 -T --trace $FILTER
err2=$?
delete_certificate
rm_image
exit $((err1 + err2))