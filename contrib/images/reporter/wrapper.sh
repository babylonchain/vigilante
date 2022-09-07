#!/usr/bin/env sh
set -euo pipefail
set -x

BINARY=/app/${BINARY:-vigilante}
LOG=${LOG:-reporter.log}
CONFIG=${CONFIG:-vigilante.yaml}

if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder. Please use the BINARY environment variable if the name of the binary is not 'vigilante'"
	exit 1
fi

export BABYLONCONFIG="/babylonconfig"
export VIGILANTECONFIG="/vigilante/${CONFIG}"
export REPORTERLOG="/vigilante/${LOG}"

if [ -d "$(dirname "${REPORTERLOG}")" ]; then
  "${BINARY}" reporter --config "${VIGILANTECONFIG}" --babylon-key "${BABYLONCONFIG}" 2>&1 | tee  "${REPORTERLOG}"
else
  "${BINARY}" reporter --config "${VIGILANTECONFIG}" --babylon-key "${BABYLONCONFIG}" 2>&1
fi
