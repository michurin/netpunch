#!/bin/sh -xe

ldflags="-s -w -X main.gitCommit=$(git rev-list --abbrev-commit -1 HEAD)"
src=./cmd/netpunch/...

if test $# = 0
then
    go build -ldflags "$ldflags" $src
    exit
fi

while test $# != 0
do
    case "$1" in
        -b)
            go build -ldflags "$ldflags" $src
            ;;
        -c)
            if test $# = 1
            then
                echo 'You have to specify os/platform'
                exit 1
            fi
            shift
            export GOOS="${1%/*}"
            export GOARCH="${1#*/}"
            go build -ldflags "$ldflags" -o netpunch-$GOOS-$GOARCH $src
            ;;
        -l)
            go tool dist list
            break
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
            echo
            echo 'Example:'
            echo './build.sh -b -c linux/386 -c darwin/amd64'
            break
            ;;
        *)
            echo 'Invalid option, try -h'
            exit 1
    esac
    shift
done
