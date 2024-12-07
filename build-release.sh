#!bash

BUILD_DIR=$(dirname "$0")/build
mkdir -p $BUILD_DIR
cd $BUILD_DIR

sum="sha1sum"

COMPRESS="gzip"
if hash pigz 2>/dev/null; then
    COMPRESS="pigz"
fi

export GO111MODULE=on
VERSION=`git describe --tags --abbrev=0`
echo "BUILDING GRASSHOPPER $VERSION" 
echo "Setting GO111MODULE to" $GO111MODULE

if ! hash sha1sum 2>/dev/null; then
    if ! hash shasum 2>/dev/null; then
        echo "I can't see 'sha1sum' or 'shasum'"
        echo "Please install one of them!"
        exit
    fi
    sum="shasum"
fi

UPX=false
if hash upx 2>/dev/null; then
    UPX=true
fi

LDFLAGS="-X main.VERSION=$VERSION -s -w"
GCFLAGS=""

# AMD64 
OSES=(linux)
for os in ${OSES[@]}; do
    suffix=""
    if [ "$os" == "windows" ]
    then
        suffix=".exe"
    fi
    env GOOS=$os GOARCH=amd64 go build -mod=vendor -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o grasshopper_${os}_amd64${suffix} github.com/xtaci/grasshopper/cmd/grasshopper
    if $UPX; then upx -9 grasshopper_${os}_amd64${suffix};fi
    tar -cf grasshopper-${os}-amd64-$VERSION.tar grasshopper_${os}_amd64${suffix}
    ${COMPRESS} -f grasshopper-${os}-amd64-$VERSION.tar
    $sum grasshopper-${os}-amd64-$VERSION.tar.gz
done
