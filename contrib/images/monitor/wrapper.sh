#!/usr/bin/env sh
set -euo pipefail
set -x

BINARY=/app/${BINARY:-vigilante}
LOG=${LOG:-monitor.log}
CONFIG=${CONFIG:-vigilante.yml}

if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder. Please use the BINARY environment variable if the name of the binary is not 'vigilante'"
	exit 1
fi

export BABYLONGENESIS="/babylon/config/genesis.json"
export VIGILANTECONFIG="/vigilante/${CONFIG}"
export MONITORLOG="/vigilante/${LOG}"

if [ -d "$(dirname "${MONITORLOG}")" ]; then
  "${BINARY}" monitor --config "${VIGILANTECONFIG}" --genesis "${BABYLONGENESIS}" 2>&1 | tee  "${MONITORLOG}"
else
  "${BINARY}" monitor --config "${VIGILANTECONFIG}" --genesis "${BABYLONGENESIS}" 2>&1
fi
