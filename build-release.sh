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

LDFLAGS="-X 'github.com/xtaci/grasshopper/cmd/grasshopper/cmd.Version=$VERSION' -s -w"
GCFLAGS=""

# AMD64 
OSES=(linux freebsd)
for os in ${OSES[@]}; do
    env CGO_ENABLED=0 GOOS=$os GOARCH=amd64 go build -mod=vendor -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o grasshopper_${os}_amd64 github.com/xtaci/grasshopper/cmd/grasshopper
    if $UPX; then upx -9 grasshopper_${os}_amd64;fi
    tar -cf grasshopper-${os}-amd64-$VERSION.tar grasshopper_${os}_amd64
    ${COMPRESS} -f grasshopper-${os}-amd64-$VERSION.tar
    $sum grasshopper-${os}-amd64-$VERSION.tar.gz
done

## Build for linux on ARM CPU
env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -mod=vendor -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o grasshopper_linux_arm github.com/xtaci/grasshopper/cmd/grasshopper
if $UPX; then upx -9 grasshopper_linux_arm;fi
tar -cf grasshopper-linux-arm-$VERSION.tar grasshopper_linux_arm
${COMPRESS} -f grasshopper-linux-arm-$VERSION.tar
$sum grasshopper-linux-arm-$VERSION.tar.gz

## Build for linux on ARM64 CPU
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -mod=vendor -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o grasshopper_linux_arm64 github.com/xtaci/grasshopper/cmd/grasshopper
if $UPX; then upx -9 grasshopper_linux_arm64;fi
tar -cf grasshopper-linux-arm64-$VERSION.tar grasshopper_linux_arm64
${COMPRESS} -f grasshopper-linux-arm64-$VERSION.tar
$sum grasshopper-linux-arm64-$VERSION.tar.gz

## Build for linux on MIPS CPU
env CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -mod=vendor -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o grasshopper_linux_mips github.com/xtaci/grasshopper/cmd/grasshopper
if $UPX; then upx -9 grasshopper_linux_mips;fi
tar -cf grasshopper-linux-mips-$VERSION.tar grasshopper_linux_mips
${COMPRESS} -f grasshopper-linux-mips-$VERSION.tar
$sum grasshopper-linux-mips-$VERSION.tar.gz

## Build for macOS
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -mod=vendor -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o grasshopper_darwin_amd64 github.com/xtaci/grasshopper/cmd/grasshopper
if $UPX; then upx -9 grasshopper_darwin_amd64;fi
tar -cf grasshopper-darwin-amd64-$VERSION.tar grasshopper_darwin_amd64
${COMPRESS} -f grasshopper-darwin-amd64-$VERSION.tar
$sum grasshopper-darwin-amd64-$VERSION.tar.gz

