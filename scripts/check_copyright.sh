#!/bin/sh
#
# Checks source files in repo for copyright notice. Adds a copyright
# notice to source files that miss it when invoked with `--fixup`.

PARALLELISM=8

pattern='Copyright \(20..-20..\|20..\), Pulumi Corporation'
notice="Copyright $(date +%Y), Pulumi Corporation.  All rights reserved."

find_unlabelled() {
    git ls-files |
	grep '[.]\(go\|ts\|cs\|py\)$' |
	xargs -L 200 -P $PARALLELISM grep "$pattern" --files-without-match
}

fixup_files() {
    comment="$1"
    while read file; do
	echo "$comment $notice\n\n$(cat $file)" > $file
    done
}

fixup() {
    unlabelled=$(find_unlabelled)
    echo $unlabelled | tr ' ' '\n' | grep '[.]\(go\|ts\|cs\)$' | fixup_files '//'
    echo $unlabelled | tr ' ' '\n' | grep '[.]py$' | fixup_files '#'
    exit 0
}

check() {
    unlabelled=$(find_unlabelled)
    n=$(($(echo $unlabelled | wc -w) + 0))

    if (( n > 0 )); then
	>&2 echo "Error: found $n source files missing a Copyright notice."
	>&2 echo "Please add a notice matching the following pattern:"
	>&2 echo "    $pattern\n"
	>&2 echo "File listing below:\n"

	echo $unlabelled | tr ' ' '\n'
	exit 1
    fi

    exit 0
}

case "$1" in
    -f|--fixup)
	fixup
	;;
    *)
	check
	;;
esac
