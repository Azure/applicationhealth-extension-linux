#!/bin/bash
set -uex
pid=$(pgrep vmwatch_linux -n)
# get avg cpu over 10 seconds
avg_cpu=$(pidstat -p $pid 1 10 | awk 'NR > 3 { sum += $8 } END { if (NR > 0) print sum / NR; else print 0 }' )

# check that cpu usage is > 0.5 and < 1.5 % as there is some wiggle room with cgroups
if (( $(echo "$avg_cpu < 1.5 && $avg_cpu > 0.5" | bc -l) )); then
  echo "PASS : avg cpu is $avg_cpu" > /var/log/azure/Extension/vmwatch-avg-cpu-check.txt
else
  echo "FAIL : avg cpu is $avg_cpu" > /var/log/azure/Extension/vmwatch-avg-cpu-check.txt
fi