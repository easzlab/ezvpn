#!/bin/bash
set -o nounset
set -o errexit
#set -o xtrace
CFSSL_VER=v1.6.3

function logger() {
  TIMESTAMP=$(date +'%Y-%m-%d %H:%M:%S')
  case "$1" in
    debug)
      echo -e "$TIMESTAMP \033[36mDEBUG\033[0m $2"
      ;;
    info)
      echo -e "$TIMESTAMP \033[32mINFO\033[0m $2"
      ;;
    warn)
      echo -e "$TIMESTAMP \033[33mWARN\033[0m $2"
      ;;
    error)
      echo -e "$TIMESTAMP \033[31mERROR\033[0m $2"
      ;;
    *)
      ;;
  esac
}

function create_certs() {
  # download
  logger info "download image: cfssl-utils"
  if [[ ! -f "$BASE/down/cfssl-utils-$CFSSL_VER.tar" ]];then
    docker pull "easzlab/cfssl-utils:$CFSSL_VER" && \
    docker save -o "$BASE/down/cfssl-utils-$CFSSL_VER.tar" "easzlab/cfssl-utils:$CFSSL_VER"
  fi
  logger debug "load image..."
  docker load -i "$BASE/down/cfssl-utils-$CFSSL_VER.tar" > /dev/null

  # clean
  docker ps -a --format="{{ .Names }}"|grep cfssl-utils > /dev/null && \
  logger info "save current certs in backup" && \
  cp -r "$BASE/certs" "$BASE/backup/certs.$(date +'%Y%m%d%H%M%S')" && \
  logger info "stop&remove container: cfssl-utils" && \
  docker rm -f cfssl-utils > /dev/null

  # create
  logger info "start container: cfssl-utils"
  docker run -d \
           --name cfssl-utils \
           --volume "$BASE/certs":/certs/ \
       "easzlab/cfssl-utils:$CFSSL_VER" \
           sleep 3600

  logger info "create ca.pem/ca-key.pem..."
  docker exec -it cfssl-utils sh -c 'cd /certs && cfssl gencert -initca ca-csr.json | cfssljson -bare ca'

  logger info "create server.pem/server-key.pem..."
  docker exec -it cfssl-utils sh -c 'cd /certs && cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=mtls server-csr.json | cfssljson -bare server'

  logger info "create agent.pem/agent-key.pem..."
  docker exec -it cfssl-utils sh -c 'cd /certs && cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=mtls agent-csr.json | cfssljson -bare agent'

}


BASE=$(cd "$(dirname "$0")"; pwd)
cd "$BASE"
mkdir -p "$BASE/down" "$BASE/certs" "$BASE/backup"

create_certs
