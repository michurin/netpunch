#!/bin/sh

ldflags="-s -w -X main.gitCommit=$(git rev-list --abbrev-commit -1 HEAD)"
src=./cmd/netpunch/...

case "$1" in
    -b)
        go build -ldflags "$ldflags" $src
        ;;
    -c)
        export GOOS="${2%/*}"
        export GOARCH="${2#*/}"
        go build -ldflags "$ldflags" -o netpunch-$GOOS-$GOARCH $src
        ;;
    -l)
        go tool dist list
        ;;
    -h)
        echo 'Usage:'
        echo '-b'
        echo '  just build'
        echo '-c os/arch'
        echo '  cross compile; see -l'
        echo '-l'
        echo '  list of available options for cross compilation'
        echo '-h'
        echo '  usage'
        ;;
    *)
        echo 'Invalid option, try -h'
esac
