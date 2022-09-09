#!/usr/bin/env sh
set -euo pipefail
set -x

BINARY=/app/${BINARY:-vigilante}
LOG=${LOG:-submitter.log}
CONFIG=${CONFIG:-vigilante.yml}

if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder. Please use the BINARY environment variable if the name of the binary is not 'vigilante'"
	exit 1
fi

export BABYLONCONFIG="/babylon"
export VIGILANTECONFIG="/vigilante/${CONFIG}"
export SUBMITTERLOG="/vigilante/${LOG}"

if [ -d "$(dirname "${SUBMITTERLOG}")" ]; then
  "${BINARY}" submitter --config "${VIGILANTECONFIG}" 2>&1 | tee  "${SUBMITTERLOG}"
else
  "${BINARY}" submitter --config "${VIGILANTECONFIG}" 2>&1
fi
