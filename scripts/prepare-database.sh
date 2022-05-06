#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

export CURRENT_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
export CASSANDRA_IMAGE="cassandra:4.0"

function check-prerequisites {
  echo "checking prerequisites"
  which docker >/dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo "docker not installed, exiting."
    exit 1
  else
    echo -n "found docker, " && docker version
  fi

  echo "checking cqlsh"
    which cqlsh >/dev/null 2>&1
    if [[ $? -ne 0 ]]; then
      echo "cqlsh not installed, exiting."
      exit 1
    else
      echo -n "found cqlsh, " && cqlsh --version
    fi
  echo "checking migrate"
    which migrate >/dev/null 2>&1
    if [[ $? -ne 0 ]]; then
      echo "migrate not installed, please refer to https://github.com/golang-migrate/migrate/tree/master/cmd/migrate for installing, exiting."
      exit 1
    else
      echo -n "found migrate, " && migrate --version
    fi
}

function cassandra-cluster-up {
  echo "running up cassandra local cluster"
  docker rm omni-cassandra --force
  docker run --name omni-cassandra --rm -p 9042:9042 -d ${CASSANDRA_IMAGE}
  echo "cassandra is running up with ip address $(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' omni-cassandra)"
  echo "waiting cassandra to be ready"
  export CQLSH_HOST=127.0.0.1
  while ! cqlsh -e 'describe cluster' ; do
      sleep 3
  done
  echo "Cassandra is ready"
  echo "Please use command : export CQLSH_HOST=127.0.0.1 && cqlsh to query database"
}

function prepare-database {
  echo "creating keyspace and table"
  ./bin/omni-orchestrator db-init --schemaFile ./database/schema/cassandra/keyspace.cql
  migrate -source file://./database/schema/cassandra/migrations --database cassandra://127.0.0.1:9042/omni_orchestrator up
}

echo "Preparing environment for omni-orchestrator developing......"

check-prerequisites

cassandra-cluster-up

prepare-database
