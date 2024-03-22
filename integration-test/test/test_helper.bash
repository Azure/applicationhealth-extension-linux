# Test helpers for BATS tests

IMAGE=applicationhealth
DOCKERFILE=test.Dockerfile
TEST_CONTAINER=test

certs_dir="$BATS_TEST_DIRNAME/certs"

_load_bats_libs() {
    export BATS_LIB_PATH=${CUSTOM_BATS_LIB_PATH:-"/usr/lib:/usr/local/lib/node_modules"}
    echo "BATS_LIB_PATH: $BATS_LIB_PATH"
    bats_load_library bats-support
    bats_load_library bats-assert
}

# This function builds a Docker image for testing purposes, if it already doesn't exist.
build_docker_image() {
    # Check if the image already exists
    if [ -z "$(docker images -q $IMAGE)" ]; then
        echo "Building test image $IMAGE..."
        docker build -q -f $DOCKERFILE -t $IMAGE . 1>&2
    else
        echo "Test image $IMAGE already exists. Skipping build."
    fi
}

in_tmp_container() {
    docker run --rm $IMAGE "$@"
}

cleanup() {
    echo "Cleaning up...">&2
    rm_container
}

rm_container() {
    echo "Deleting test container $TEST_CONTAINER ...">&2 && \
    docker rm -f $TEST_CONTAINER &>/dev/null && \
        echo "Deleted test container." || true
}

# Function to delete a Docker image.
# Usage: rm_image
# Returns: None
rm_image() {
    local image_id=$(docker images -q $IMAGE)
    echo "Deleting Docker Image ID: $image_id ...">&2 && \
    docker rmi -f $image_id &>/dev/null && \
        echo "Deleted test image." || true
}

mk_container() {
     if [ $# -gt 3 ]; then # if less than two arguments are supplied
        local container_name="${1:-$TEST_CONTAINER}" # assign the value of $TEST_CONTAINER if $1 is empty
        echo "container_name: $container_name"
        TEST_CONTAINER="$container_name"
        shift
    fi

    rm_container && echo "Creating test container with commands: $@">&2 && \
        docker create --name=$TEST_CONTAINER $IMAGE "$@" 1>/dev/null
}

# creates a container in priviged mode (allowing cgroup integration to work)
mk_container_priviliged() {
    if [ $# -gt 3 ]; then # if less than two arguments are supplied
        local container_name="${1:-$TEST_CONTAINER}" # assign the value of $TEST_CONTAINER if $1 is empty
        echo "container_name: $container_name"
        TEST_CONTAINER="$container_name"
        shift
    fi

    rm_container && echo "Creating test container with commands: $@">&2 && \
        docker create --privileged --name=$TEST_CONTAINER $IMAGE "$@" 1>/dev/null
}

in_container() {
    set -e
    rm_container
    mk_container "$@"
    start_container
}

start_container() {
    echo "Starting test container...">&2 && docker start --attach $TEST_CONTAINER
}

container_diff() {
    docker diff $TEST_CONTAINER
}

container_read_file() { # reads the file at container path $1
    set -eo pipefail
    docker cp $TEST_CONTAINER:"$1" - | tar x --to-stdout
}

container_read_extension_status() {
    container_read_file /var/lib/waagent/Extension/status/0.status
}

container_read_vmwatch_log() {
    container_read_file /var/log/azure/Extension/VE.RS.ION/vmwatch.log
}

container_read_handler_log() {
    container_read_file /var/log/azure/applicationhealth-extension/handler.log
}

mk_certs() { # creates certs/{THUMBPRINT}.(crt|key) files under ./certs/ and prints THUMBPRINT
    set -eo pipefail
    mkdir -p "$certs_dir" && cd "$certs_dir" && rm -f "$certs_dir/*"
    openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -batch &>/dev/null
    thumbprint=$(openssl x509 -in cert.pem -fingerprint -noout| sed 's/.*=//g' | sed 's/://g')
    mv cert.pem $thumbprint.crt && \
        mv key.pem $thumbprint.prv
    echo $thumbprint
}

push_certs() { # pushes certs/{$1}.(crt|key) files to container
    set -e
    docker cp "$certs_dir/$1.crt" $TEST_CONTAINER:/var/lib/waagent/
    docker cp "$certs_dir/$1.prv" $TEST_CONTAINER:/var/lib/waagent/
    echo "Pushed certs to container.">&2
}

encrypt_settings(){ # encrypts the message ($2) with the key with given cert thumbprint ($1)
    set -eo pipefail
    tp="$1"; msg="$2"
    echo "$(openssl smime -inkey "$certs_dir/$tp.prv" -encrypt -outform DER "$certs_dir/$tp.crt" < <(echo "$msg") | base64 -w0)" 
}

mk_settings_json() { # turns json public settings ($1) and ($2) into a json encrypted with "$3".(crt|prv)
    set -e
    pub="$1"
    prot="$2"
    cert_tp="$3"
    if [ -z "$pub" ]; then pub="null"; fi    
    if [ -n "$prot" ]; then
        prot="\"$(encrypt_settings "$cert_tp" "$prot")\""
    else
        cert_tp="null"
        prot="null"
    fi
    
    cat <<-EOF
    {
        "runtimeSettings": [
            {
                "handlerSettings": {
                    "protectedSettingsCertThumbprint": "$cert_tp",
                    "publicSettings": $pub,
                    "protectedSettings": $prot
                }
            }
        ]
    }
EOF
}

push_settings() { # creates and copies 0.settings file with given public settings ($1) and ($2) values.
    set -e

    if [ -n "$2" ]; then
        cert_tp="$(mk_certs)"
        push_certs "$cert_tp"
    fi

    cfg_file="$(save_tmp_file "$(mk_settings_json "$1" "$2" "$cert_tp") ")"
    echo ".settings: $(cat "$cfg_file")" >&2
    copy_config "$cfg_file"
    echo ".settings file pushed to container." >&2
}

save_tmp_file(){ # saves $1 into a temporary file and returns its path
    fp="$(mktemp)"
    touch "$fp"
    cat <<< "$1" > "$fp"
    echo "$fp"
}

copy_config() { # places specified settings file ($1) into container as 0.settings
    set -e
    echo "Copying $1 to container as 0.settings." >&2
    docker cp "$1" "$TEST_CONTAINER:/var/lib/waagent/Extension/config/0.settings"
    echo "Copied settings into container.">&2
}

# first argument is the string containing healthextension logs separated by newline
# it also expects the time={time in TZ format} level... to be in each log line
# second argument is an array of expected time difference (in seconds) between previous log
# for example: [5,10] means that the expected time difference between second log and first log is 5 seconds
# and time difference between third log and second log is 10 seconds
verify_state_change_timestamps() {
    expectedTimeDifferences="$2"
    regex='time=([^[:space:]]*)' # regex to extract time from log line, will select everything until a space is found
    prevDate=""
    index=0
    while IFS=$'\n' read -ra enableLogs; do
        for i in "${!enableLogs[@]}"; do
            [[ $enableLogs[index] =~ $regex ]]
            currentDate=${BASH_REMATCH[1]}
            if [[ ! -z "$prevDate" ]]; then
                diff=$(( $(date -d "$currentDate" "+%s") - $(date -d "$prevDate" "+%s") ))
                echo "Actual time difference is: $diff and expected is: ${expectedTimeDifferences[$index-1]}"
                [[ "$diff" -ge "${expectedTimeDifferences[$index-1]}" ]]
            fi
        index=$index+1
        prevDate=$currentDate
        done
    done <<< "$1"
}

# first argument is the string containing healthextension logs separated by newline
# it also expects event={"description of state event"} to be in each log line
# second argument is an array of expected state log strings
verify_states() {
    expectedStateLogs="$2"
    regex='msg="(.*)"'
    index=0
    while IFS=$'\n' read -ra stateLogs; do
        for i in "${!stateLogs[@]}"; do
            [[ $stateLogs[i] =~ $regex ]]
            stateEvent=${BASH_REMATCH[1]}
            echo "Actual: '$stateEvent' and expected is: '${expectedStateLogs[index]}'"
            [[ "$stateEvent" == "${expectedStateLogs[index]}" ]]
        index=$index+1
        done
    done <<< "$1"
}

verify_status_item() {
    # $1 status_file contents
    # $2 status.operation
    # $3 status.status 
    # $4 status.formattedMessage.message
    #       Note that this can contain regex 
    FMT='"operation": "'%s'",((.*)|\s*?).*,\s*"status": "'%s'",\s+"formattedMessage": {\s+"lang": "en",\s+"message": "'%s'"'
    printf -v STATUS "$FMT" "$2" "$3" "$4"
    echo "Searching status file for status item: $STATUS"
    echo "$1" | egrep -z "$STATUS"
}

verify_substatus_item() {
    # $1 status_file contents
    # $2 substatus.name
    # $3 substatus.status 
    # $4 substatus.formattedMessage.message
    #       Note that this can contain regex 
    FMT='"name": "'%s'",\s+"status": "'%s'",\s+"formattedMessage": {\s+"lang": "en",\s+"message": "'%s'"'
    printf -v SUBSTATUS "$FMT" "$2" "$3" "$4"
    echo "Searching status file for substatus item: $SUBSTATUS"
    echo "$1" | egrep -z "$SUBSTATUS"
}

create_certificate() {
    # Create a random seed file
    openssl rand -out ~/.rnd 2048

    # Generate a private key
    openssl genrsa -out testbin/webserverkey.pem 2048

    # Generate a self-signed certificate
    openssl req -new -x509 -sha256 -key testbin/webserverkey.pem -out testbin/webservercert.pem -days 3650 -subj '/CN=www.contoso.com/O=Contoso LTD./C=US'
}

delete_certificate() {
    rm -f testbin/webserverkey.pem
    rm -f testbin/webservercert.pem
}

get_extension_version() {
    # extract version from manifest.xml
    version=$(awk -F'[<>]' '/<Version>/ {print $3}' misc/linux/manifest.xml)
    echo $version
}
