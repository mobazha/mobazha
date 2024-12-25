#!/bin/bash

## Version 3.0.0
##
## Usage
## ./build.sh <platform>
##
## OS supported:
## win32 win64 linux32 linux64 linuxarm osx
##

ELECTRONVER=22.0.0

# Get Version
PACKAGE_VERSION=$(node -p 'require("./package").version')
echo "Mobazha Version: $PACKAGE_VERSION"

echo 'Preparing to build installers...'

echo 'Downloading Mobazha node...'

mkdir -p build/extraResources/mobazha

if [ "$1" == "linux" ]; then
  if [ ! -f build/extraResources/mobazha/mobazhad ]; then
    curl -L -o build/extraResources/mobazha/mobazhad https://github.com/mobazha/mobazha-go/releases/latest/download/mobazhad-linux-amd64
    chmod +x build/extraResources/mobazha/mobazhad
  else
    echo "Mobazha node already exists for Linux"
  fi
elif [ "$1" == "osx" ]; then
  if [ ! -f build/extraResources/mobazha/mobazhad ]; then
    curl -L -o build/extraResources/mobazha/mobazhad https://github.com/mobazha/mobazha-go/releases/latest/download/mobazhad-darwin-amd64
    chmod +x build/extraResources/mobazha/mobazhad
  else
    echo "Mobazha node already exists for macOS"
  fi
elif [ "$1" == "win" ]; then
  if [ ! -f build/extraResources/mobazha/mobazha.exe ]; then
    curl -L -o build/extraResources/mobazha/mobazha.exe https://github.com/mobazha/mobazha-go/releases/latest/download/mobazha.exe
  else
    echo "Mobazha node already exists for Windows"
  fi
fi

echo 'Installing npm packages...'
npm install

echo 'Installing frontend dependencies...'
cd frontend
npm install
cd ..

echo 'Building Mobazha app...'
npm run clean
npm run encrypt
npm run build-frontend

echo "We are building for platform: ${1}"

case "$1" in
  "linux")
    echo 'Building Linux Installer...'
    npm run build-l
    ;;
  "osx")
    echo 'Building macOS Installer...'
    npm run build-m
    ;;
  "win")
    echo 'Building Windows Installer...'
    npm run build-w
    ;;
  *)
    echo "Unsupported platform: ${1}"
    exit 1
    ;;
esac
