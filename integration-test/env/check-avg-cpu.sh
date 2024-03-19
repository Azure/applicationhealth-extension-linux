#!/bin/bash
set -uex

process_name=$1
min_cpu=$2
max_cpu=$3

pid=$(pgrep $process_name -n)
# get avg cpu over 10 seconds
avg_cpu=$(pidstat -p $pid 1 10 | awk 'NR > 3 { sum += $8 } END { if (NR > 0) print sum / NR; else print 0 }' )

# check that cpu usage is > min_cpu and < max_cpu % as there is some wiggle room with cgroups
if (( $(echo "$avg_cpu > $min_cpu && $avg_cpu < $max_cpu" | bc -l) )); then
  echo "PASS : avg cpu is $avg_cpu" > /var/log/azure/Extension/vmwatch-avg-cpu-check.txt
else
  echo "FAIL : avg cpu is $avg_cpu" > /var/log/azure/Extension/vmwatch-avg-cpu-check.txt
fi