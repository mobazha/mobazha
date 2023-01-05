#!/bin/bash

## Version 3.0.0
##
## Usage
## ./build.sh
##
## OS supported:
## win32 win64 linux32 linux64 linuxarm osx
##


ELECTRONVER=6.0.0
NODEJSVER=14.21.2

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

    # brew update
    # brew remove jq
    # brew link oniguruma
    # brew install jq
    # brew link --overwrite fontconfig gd gnutls jasper libgphoto2 libicns libtasn1 libusb libusb-compat little-cms2 nettle openssl sane-backends webp wine git-lfs gnu-tar dpkg xz
    # brew install freetype graphicsmagick
    # brew link xz
    # brew remove openssl
    # brew install openssl
    # brew link freetype graphicsmagick mono

    # # Retrieve Latest Server Binaries
    # cd MOBAZHA_TEMP/
    # curl -u $GITHUB_USER:$GITHUB_TOKEN -s https://api.github.com/repos/OpenBazaar/openbazaar-go/releases/$SERVERTAG > release.txt
    # cat release.txt | jq -r ".assets[].browser_download_url" | xargs -n 1 curl -L -O
    # cd ..

    if [[ $BINARY == 'win' ]]; then

        # brew link --overwrite fontconfig gd gnutls jasper libgphoto2 libicns libtasn1 libusb libusb-compat little-cms2 nettle openssl sane-backends webp wine git-lfs gnu-tar dpkg xz
        # brew link libgsf glib pcre
        # brew remove osslsigncode
        # brew install mono osslsigncode
        # brew reinstall openssl@1.1

        # brew install homebrew/cask-versions/wine-devel

        # WINDOWS 64
        echo 'Building Windows 64-bit Installer...'
        mkdir dist/win64

        export WINEARCH=win64

        npm i electron-packager

        cd node_modules/electron-packager
        npm install rcedit
        cd ../..

        echo 'Running Electron Packager...'
        node_modules/electron-packager/bin/electron-packager.js . Mobazha --asar --out=dist --protocol-name=Mobazha --ignore="MOBAZHA_TEMP" --win32metadata.ProductName="Mobazha" --win32metadata.CompanyName="Mogaolei" --win32metadata.FileDescription='Decentralized p2p marketplace for Bitcoin' --win32metadata.OriginalFilename=Mobazha.exe --protocol=ob --platform=win32 --arch=x64 --icon=imgs/openbazaar2.ico --electron-version=${ELECTRONVER} --overwrite

        echo 'Copying server binary into application folder...'
        mkdir -p dist/Mobazha/resources/mobazha dist/Mobazha/resources/app
        cp -rf MOBAZHA_TEMP/mobazha-amd64.exe dist/Mobazha/resources/mobazha/mobazhad.exe
        cp -rf MOBAZHA_TEMP/libwinpthread-1.win64.dll dist/Mobazha/resources/mobazha/libwinpthread-1.dll
        
        echo 'Building Installer...'
        grunt -v create-windows-installer --appname=Mobazha --obversion=$PACKAGE_VERSION --appdir=dist/Mobazha --outdir=dist/win64
        mv dist/win64/MobazhaSetup.exe dist/win64/Mobazha-$PACKAGE_VERSION-Setup-64.exe
        mv dist/win64/RELEASES dist/win64/RELEASES-x64

        #### CLIENT ONLY
        echo 'Running Electron Packager...'
        electron-packager . MobazhaClient --asar --out=dist --protocol-name=Mobazha --ignore="MOBAZHA_TEMP" --win32metadata.ProductName="MobazhaClient" --win32metadata.CompanyName="Mogaolei" --win32metadata.FileDescription='Decentralized p2p marketplace for Bitcoin' --win32metadata.OriginalFilename=MobazhaClient.exe --protocol=ob --platform=win32 --arch=x64 --icon=imgs/openbazaar2.ico --electron-version=${ELECTRONVER} --overwrite

        echo 'Building Installer...'
        grunt -v create-windows-installer --appname=MobazhaClient --obversion=$PACKAGE_VERSION --appdir=dist/MobazhaClient-win32-x64 --outdir=dist/win64
        mv dist/win64/MobazhaClientSetup.exe dist/win64/MobazhaClient-$PACKAGE_VERSION-Setup-64.exe

        echo 'Sign the installer'
        osslsigncode sign -t http://timestamp.sectigo.com -h sha1 -key .travis/mobazha.com.key -certs .travis/mobazha.com.crt -in dist/win64/Mobazha-$PACKAGE_VERSION-Setup-64.exe -out dist/win64/Mobazha-$PACKAGE_VERSION-Setup-64.exe
        osslsigncode sign -t http://timestamp.sectigo.com -h sha1 -key .travis/mobazha.com.key -certs .travis/mobazha.com.crt -in dist/win64/MobazhaClient-$PACKAGE_VERSION-Setup-64.exe -out dist/win64/MobazhaClient-$PACKAGE_VERSION-Setup-64.exe

        mv dist/win64/RELEASES-x64 dist/win64/RELEASES

    else

        # OSX
        echo 'Building OSX Installer'
        mkdir dist/osx

        # Install the DMG packager
        echo 'Installing electron-installer-dmg'
        npm install -g electron-installer-dmg

        # Sign mobazha binary
        echo 'Signing Go binary'
        mv MOBAZHA_TEMP/mobazha-darwin-10.6-amd64 dist/osx/mobazhad
        # rm -rf MOBAZHA_TEMP/*
        codesign --force --sign "$SIGNING_IDENTITY2" --timestamp --options runtime dist/osx/mobazhad

        # Notarize the zip files
        UPLOAD_INFO_PLIST="uploadinfo.plist"
        REQUEST_INFO_PLIST="request.plist"
        touch ${UPLOAD_INFO_PLIST}

        wait_for_notarization() {
          while true; do \

            echo "Checking Apple for notarization status..."; \
            /usr/bin/xcrun altool --notarization-info `/usr/libexec/PlistBuddy -c "Print :notarization-upload:RequestUUID" $UPLOAD_INFO_PLIST` -u $APPLE_ID -p $APPLE_PASS --output-format xml > "$REQUEST_INFO_PLIST" ;\

            cat $REQUEST_INFO_PLIST

            if [[ `/usr/libexec/PlistBuddy -c "Print :notarization-info:Status" ${REQUEST_INFO_PLIST}` != "in progress" ]] || [[ "$requestUUID" == "" ]] ; then \

               # check if it has been uploaded already and get the RequestUUID from the error message
               echo "Checking if binary has already been uploaded..."; \
               message=`/usr/libexec/PlistBuddy -c "Print :product-errors:0:message" $UPLOAD_INFO_PLIST`;\
               if [[ ${message} =~ ^ERROR\ ITMS-90732* ]]; then \
                   prefix="ERROR ITMS-90732: \"The software asset has already been uploaded. The upload ID is "; \
                   suffix="\" at SoftwareAssets\/EnigmaSoftwareAsset"; \
                   requestUUID=`echo "${message}" | sed -e "s/^$prefix//" -e "s/$suffix$//"`; \

                   echo "Binary has already been uploaded. Checking Apple status for request ${requestUUID}..."; \
                   /usr/bin/xcrun altool --notarization-info ${requestUUID} -u $APPLE_ID -p $APPLE_PASS --output-format xml > "$REQUEST_INFO_PLIST" ;\
               fi ;\

               if [[ `/usr/libexec/PlistBuddy -c "Print :notarization-info:Status" ${REQUEST_INFO_PLIST}` == "success" ]]; then \
                echo "Binary has been notarized"; \
                break; \
               fi; \
            fi ;\
            echo "Waiting 30 seconds to check status again..."; \
            sleep 30 ;\
          done
        }

        extract_app() {

            # use process redirection to capture the mount point and dev entry
            IFS=$'\n' read -rd '\n' mount_point dev_entry < <(
                # mount the diskimage; leave out -readonly if making changes to the filesystem
                hdiutil attach -readonly -plist "$1" | \

                # convert output plist to json
                plutil -convert json - -o - | \

                # extract mount point and dev entry
                jq -r '
                    .[] | .[] |
                    select(."volume-kind" == "hfs") |
                    ."mount-point" + "\n" + ."dev-entry"
                '
            )

            # work with the zip file
            cp -rf "${mount_point}/${2}.app" dist/osx

            # unmount the disk image
            hdiutil detach "$dev_entry"

        }

        if [[ ${BINARY} == 'osx' ]]; then

            echo 'Running Electron Packager...'
            electron-packager . Mobazha --out=dist -app-category-type=public.app-category.business --protocol-name=Mobazha --ignore="MOBAZHA_TEMP" --protocol=ob --platform=darwin --arch=x64 --icon=imgs/openbazaar2.icns --electron-version=${ELECTRONVER} --overwrite --app-version=$PACKAGE_VERSION

            echo 'Creating mobazha folder in the OS X .app'
            mkdir dist/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/mobazha

            echo 'Moving binary to correct folder'
            mv dist/osx/mobazhad dist/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/mobazha/mobazhad
            chmod +x dist/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/mobazha/mobazhad

            echo 'Codesign the .app'
            codesign -s "$SIGNING_IDENTITY2" dist/Mobazha-darwin-x64/Mobazha.app/Contents/Frameworks/Electron\ Framework.framework/Versions/A/Libraries/libffmpeg.dylib
            codesign -s "$SIGNING_IDENTITY2" dist/Mobazha-darwin-x64/Mobazha.app/Contents/Frameworks/Electron\ Framework.framework/Versions/A/Libraries/libnode.dylib
            codesign --force --options runtime --deep --sign "$SIGNING_IDENTITY2" "dist/Mobazha-darwin-x64/Mobazha.app/Contents/Frameworks/Electron Framework.framework/Versions/A/Resources/crashpad_handler"
            codesign --force --options runtime --deep --sign "$SIGNING_IDENTITY2"  "dist/Mobazha-darwin-x64/Mobazha.app/Contents/Frameworks/Squirrel.framework/Versions/A/Resources/ShipIt"

            codesign --force --deep --sign "$SIGNING_IDENTITY2" --timestamp --options runtime --entitlements openbazaar.entitlements dist/Mobazha-darwin-x64/Mobazha.app
            electron-installer-dmg dist/Mobazha-darwin-x64/Mobazha.app Mobazha-$PACKAGE_VERSION --icon ./imgs/openbazaar2.icns --out=dist/Mobazha-darwin-x64 --overwrite --background=./imgs/osx-finder_background.png --debug

            echo 'Codesign the DMG and zip'
            codesign --force --sign "$SIGNING_IDENTITY2" --timestamp --options runtime --entitlements openbazaar.entitlements dist/Mobazha-darwin-x64/Mobazha-$PACKAGE_VERSION.dmg
            cd dist/Mobazha-darwin-x64/
            zip -q -r Mobazha-mac-$PACKAGE_VERSION.zip Mobazha.app
            cp -r Mobazha.app ../osx/
            cp Mobazha-mac-$PACKAGE_VERSION.zip ../osx/
            cp Mobazha-$PACKAGE_VERSION.dmg ../osx/

            cd ../..

            zip -q -r dist/osx/Mobazha.zip dist/Mobazha-darwin-x64/Mobazha-$PACKAGE_VERSION.dmg

            # Upload to apple and notarize
            echo "Uploading binary to Apple Notarization server for package ${PACKAGE_VERSION}..."
            xcrun altool --notarize-app --primary-bundle-id "org.openbazaar.desktop-${PACKAGE_VERSION}" --username "$APPLE_ID" --password "$APPLE_PASS" --file dist/osx/Mobazha.zip --output-format xml > ${UPLOAD_INFO_PLIST}
            wait_for_notarization

            echo "Stapling ticket to the DMG..."
            xcrun stapler staple dist/osx/Mobazha-$PACKAGE_VERSION.dmg

            extract_app "dist/osx/Mobazha-$PACKAGE_VERSION.dmg" "Mobazha"

            zip -q -r dist/osx/Mobazha-mac-$PACKAGE_VERSION.zip dist/osx/Mobazha.app

        else

            # Client Only
            electron-packager . MobazhaClient --out=dist -app-category-type=public.app-category.business --protocol-name=Mobazha --ignore="MOBAZHA_TEMP" --protocol=ob --platform=darwin --arch=x64 --icon=imgs/openbazaar2.icns --electron-version=${ELECTRONVER} --overwrite --app-version=$PACKAGE_VERSION

            codesign -s "$SIGNING_IDENTITY2" dist/MobazhaClient-darwin-x64/MobazhaClient.app/Contents/Frameworks/Electron\ Framework.framework/Versions/A/Libraries/libffmpeg.dylib
            codesign -s "$SIGNING_IDENTITY2" dist/MobazhaClient-darwin-x64/MobazhaClient.app/Contents/Frameworks/Electron\ Framework.framework/Versions/A/Libraries/libnode.dylib
            codesign --force --options runtime --deep --sign "$SIGNING_IDENTITY2" "dist/MobazhaClient-darwin-x64/MobazhaClient.app/Contents/Frameworks/Electron Framework.framework/Versions/A/Resources/crashpad_handler"
            codesign --force --options runtime --deep --sign "$SIGNING_IDENTITY2"  "dist/MobazhaClient-darwin-x64/MobazhaClient.app/Contents/Frameworks/Squirrel.framework/Versions/A/Resources/ShipIt"

            codesign --force --deep --sign "$SIGNING_IDENTITY2" --timestamp --options runtime --entitlements openbazaar.entitlements dist/MobazhaClient-darwin-x64/MobazhaClient.app
            electron-installer-dmg dist/MobazhaClient-darwin-x64/MobazhaClient.app MobazhaClient-$PACKAGE_VERSION --icon ./imgs/openbazaar2.icns --out=dist/MobazhaClient-darwin-x64 --overwrite --background=./imgs/osx-finder_background.png --debug

            # Client Only
            codesign --force --sign "$SIGNING_IDENTITY2" --timestamp --options runtime --entitlements openbazaar.entitlements dist/MobazhaClient-darwin-x64/MobazhaClient-$PACKAGE_VERSION.dmg
            cd dist/MobazhaClient-darwin-x64/
            zip -q -r MobazhaClient-mac-$PACKAGE_VERSION.zip MobazhaClient.app
            cp -r MobazhaClient.app ../osx/
            cp MobazhaClient-mac-$PACKAGE_VERSION.zip ../osx/
            cp MobazhaClient-$PACKAGE_VERSION.dmg ../osx/

            cd ../..

            zip -q -r dist/osx/MobazhaClient.zip dist/MobazhaClient-darwin-x64/MobazhaClient-$PACKAGE_VERSION.dmg

            echo "Uploading client only binary to Apple Notarization server..."
            xcrun altool --notarize-app --primary-bundle-id "org.openbazaar.desktopclient-$PACKAGE_VERSION" --username "$APPLE_ID" --password "$APPLE_PASS" --file dist/osx/MobazhaClient.zip --output-format xml > $UPLOAD_INFO_PLIST
            wait_for_notarization

            echo "Stapling ticket to the DMG..."
            xcrun stapler staple dist/osx/MobazhaClient-$PACKAGE_VERSION.dmg

            extract_app "dist/osx/MobazhaClient-$PACKAGE_VERSION.dmg" "MobazhaClient"

            zip -q -r dist/osx/MobazhaClient-mac-$PACKAGE_VERSION.zip dist/osx/MobazhaClient.app
        fi

    fi

  ;;
esac
