#!/bin/bash

## Version 3.0.0
##
## Usage
## ./build.sh
##
## OS supported:
## win32 win64 linux32 linux64 linuxarm osx
##


ELECTRONVER=22.0.0

BINARY="${1}"
TRAVIS_OS_NAME="osx"
# if [ -z "${2}" ]; then
# SERVERTAG='latest'
# else
# SERVERTAG=tags/${2}
# fi
# echo "Building with mobazha/$SERVERTAG"

# Get Version
PACKAGE_VERSION=$(node -p 'require("./package").version')
echo "Mobazha Version: $PACKAGE_VERSION"

# Create temp build dirs
# mkdir dist/
# rm -rf dist/*
# mkdir MOBAZHA_TEMP/
# rm -rf MOBAZHA_TEMP/*

echo 'Preparing to build installers...'

echo 'Installing npm packages...'
#npm i -g npm@5.2
# npm install electron-packager -g --silent
# npm install npm-run-all -g --silent
# npm install grunt-cli -g --silent
# npm install grunt --save-dev --silent
# npm install grunt-electron-installer --save-dev --silent
# npm install --silent

# rvm reinstall ruby

echo 'Building Mobazha app...'
npm run build

echo 'Copying transpiled files into js folder...'
cp -rf prod/* js/

echo "We are building: ${BINARY}"

case "$TRAVIS_OS_NAME" in
  "linux")

    echo 'Linux builds'
    echo 'Making dist directories'
    mkdir dist/linux64

    sudo apt-get install rpm

    echo 'Install npm packages for Linux'
    npm install -g --save-dev electron-installer-debian --silent
    npm install -g --save-dev electron-installer-redhat@2.0.0 --silent

    # Install libgconf2-4
    sudo apt-get install libgconf2-4 libgconf-2-4

    # Install rpmbuild
    sudo apt-get --only-upgrade install rpm

    # Ensure fakeroot is installed
    sudo apt-get install fakeroot

    # # Retrieve Latest Server Binaries
    # sudo apt-get install jq
    # cd MOBAZHA_TEMP/
    # curl -u $GITHUB_USER:$GITHUB_TOKEN -s https://api.github.com/repos/OpenBazaar/openbazaar-go/releases/$SERVERTAG > release.txt
    # cat release.txt | jq -r ".assets[].browser_download_url" | xargs -n 1 curl -L -O
    # cd ..

    APPNAME="mobazha"

    echo 'Building Linux 64-bit Installer....'

    echo "Packaging Electron application"
    electron-packager . ${APPNAME} --platform=linux --arch=x64 --electronVersion=${ELECTRONVER} --overwrite --ignore="MOBAZHA_TEMP" --prune --out=dist

    echo 'Move go server to electron app'
    mkdir dist/${APPNAME}-linux-x64/resources/mobazha/
    cp -rf MOBAZHA_TEMP/mobazha-linux-amd64 dist/${APPNAME}-linux-x64/resources/mobazha
    rm -rf MOBAZHA_TEMP/*
    mv dist/${APPNAME}-linux-x64/resources/mobazha/mobazha-linux-amd64 dist/${APPNAME}-linux-x64/resources/mobazha/mobazhad
    rm -rf dist/${APPNAME}-linux-x64/resources/app/.travis
    chmod +x dist/${APPNAME}-linux-x64/resources/mobazha/mobazhad

    echo 'Create debian archive'
    electron-installer-debian --config .travis/config_amd64.json

    echo 'Create RPM archive'
    electron-installer-redhat --config .travis/config_x86_64.json

    APPNAME="mobazhaclient"

    echo 'Building Linux 64-bit Installer....'

    echo "Packaging Electron application"
    electron-packager . ${APPNAME} --platform=linux --arch=x64 --ignore="MOBAZHA_TEMP" --electronVersion=${ELECTRONVER} --overwrite --prune --out=dist

    echo 'Create debian archive'
    electron-installer-debian --config .travis/config_amd64.client.json

    echo 'Create RPM archive'
    electron-installer-redhat --config .travis/config_x86_64.client.json

    ;;

  "osx")
    if [[ $BINARY == 'win' ]]; then
        echo 'Running Electron Packager...'
        npm run package

        echo 'Copying server binary into application folder...'
        mkdir -p dist/Mobazha/resources/mobazha
        cp -rf MOBAZHA_TEMP/mobazha-amd64.exe dist/Mobazha/resources/mobazha/mobazhad.exe
        
        npm run make
    else
        # OSX
        echo 'Building OSX Installer'
        mkdir dist/osx

        # Sign mobazha binary
        echo 'Signing Go binary'
        cp MOBAZHA_TEMP/mobazha-darwin-amd64 dist/osx/mobazhad
        # rm -rf MOBAZHA_TEMP/*
        # codesign --force --sign "$SIGNING_IDENTITY2" --timestamp --options runtime dist/osx/mobazhad

        echo 'Running Electron Packager...'
        npm run package

        echo 'Creating mobazha folder in the OS X .app'
        mkdir out/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/mobazha

        echo 'Moving binary to correct folder'
        cp MOBAZHA_TEMP/mobazha-darwin-amd64 out/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/mobazha/mobazhad
        chmod +x out/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/mobazha/mobazhad
        
        npm run make
    fi

  ;;
esac
