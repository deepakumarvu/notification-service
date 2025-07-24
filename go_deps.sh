#!/bin/bash

# Usage go_deps.sh abc_main.go build/abc/bootstrap

DEPS_FILE=$2.deps
TMP_FILE=$DEPS_FILE.tmp

mkdir -p `dirname $2`
LIST=`echo -e "$2: $1 $DEPS_FILE"`

for i in `go list -f {{.Deps}} $1 | tr ' ' '\n' | sort | uniq | grep rmng | sed -e 's/rmng\///g'`
do
	if [ -d $i ]; then
		LIST="$LIST `ls $i/*.go | tr '\n' ' '`"
	fi
done
echo $LIST > $TMP_FILE
mv $TMP_FILE $DEPS_FILE
