#!/bin/sh

pf=${1}

workdir=$(cd $(dirname $0); pwd)
out=$workdir/hot-svr
export GOARCH=amd64
export CGO_ENABLED=0

if [ "$pf" = "windows" ];then
    export GOOS=windows
    out="$out-win.exe"
elif [ "$pf" = "linux" ];then
    export GOOS=linux
    out="$out-linux"
else
    export GOOS=darwin
    out="$out-osx"
fi

go build -ldflags "-w -s" -gcflags="all=-trimpath=${PWD}" -asmflags="all=-trimpath=${PWD}" -o $out $workdir/.