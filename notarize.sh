PACKAGE_VERSION="3.0.0"

#codesign --sign 5A8996824DE3C96CCD5283D8603B12917D0F9DC --force --timestamp --options runtime --entitlements ./node_modules/@electron/osx-sign/entitlements/default.darwin.plist out/Mobazha-darwin-x64/Mobazha.app/Contents/Resources/app/mobazha/mobazhad

zip -q -r out/Mobazha.zip out/make/Mobazha-3.0.0-x64.dmg

# Upload to apple and notarize
echo "Uploading binary to Apple Notarization server for package ${PACKAGE_VERSION}..."
xcrun notarytool submit out/Mobazha.zip --keychain-profile "MOBAZHA_NOTARY"  --wait

# xcrun notarytool history --keychain-profile "MOBAZHA_NOTARY"

# xcrun notarytool log df90ee05-67bc-4cc9-bacd-472f2ed5d05c --keychain-profile "MOBAZHA_NOTARY"

#echo "Stapling ticket to the DMG..."
#xcrun stapler staple out/make/Mobazha-3.0.0-x64.dmg
