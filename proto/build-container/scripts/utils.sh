#!/usr/bin/env bash

# ensureSet tests a variable to ensure it has a value. If there is a value, 0 is
# returned. If it does not, a message is printed to stderr, and 1 is returned.
#
# Callers determine whether to exit their script as a result of this.
#
# Example usage:
#
#      ensureSet "${AWS_ACCESS_KEY_ID}" "AWS_ACCESS_KEY_ID" || exit 1
function ensureSet() {
	local variable=$1
	local description=$2

	if [[ -n "${variable}" ]]; then
		return 0
	else
		echo "error: ${description} has no value"
		return 1
	fi
}

# verifySHASUM computes the SHA256 of the file at `filepath`, and compares it
# with the value of `expected`. If they match, 0 is returned. If they do not,
# a message detailing the mismatch is printed to stderr, and 1 is returned.
#
# Callers determine whether to exit their script as a result of this.
#
# Example usage:
#
#	EXPECTED=5ddc65f205c1cd88287f460e0c7e7fc87662987d1f1aec1e25b6479e5a5a08cb
#	verifySHASUM "/tmp/file.zip" "${EXPECTED}"
function verifySHASUM() {
	local filepath=$1
	local expected=$2

	local actual
	actual=$(sha256sum "${filepath}" | awk '{print $1}')

	if [[ "${expected}" == "${actual}" ]]; then
		return 0
	else
		echo "SHA mismatch for ${filepath}:"
		echo "  Expected: ${expected}"
		echo "       Got: ${actual}"
		return 1
	fi
}