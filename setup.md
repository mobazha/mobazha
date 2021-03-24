
### Setup Instructions:

1. Clone the repo
  - https://github.com/OpenBazaar/haven
2. Install npm and pod dependencies

```
cd haven
yarn
cd ios && pod install
```

3. Copy `.env` file to the root directory
  - If you want to run the prod target, the env file name should be `.env.prod`. The env file should look like this:

```
BRANCH_KEY=
COUNTLY_ROOT_URL=
COUNTLY_APP_KEY=
STREAM_API_KEY=
STREAM_APP_ID=
HMAC_SECRET=
```

4. Copy those files into `havenBuildConfigFiles` folder:

```
AppCenter-Config.plist
GoogleService-Info.plist
appcenter-config.json
google-services.json
```

5. In `havenBuildConfigs` folder, run `copy_files.sh`
6. Run the app: `react-native run-ios`

#### iOS
Then you'll need to set up iOS simulator (Xcode). The app can also be installed on your device, but the default is not a static build, meaning it will need to be connected to Xcode to function. Static build instructions can be found [here](http://facebook.github.io/react-native/docs/running-on-device.html).

In Xcode, run the project on a simulator or install on your device.

#### Android

Setup Android USB debugging mode and make sure `adb` is installed on your computer. Plug in your Android device into your computer, make sure that your computer recognizes the device (type in `adb devices`), and run: `react-native run-android`.

Again, static builds for Android haven't been compiled yet, but further instructions are [here](http://facebook.github.io/react-native/docs/running-on-device.html).

##### Building Android

The application most easily builds with:

- Android Studio 3.1 (https://developer.android.com/studio/archive)
- Android SDK Build Tools v27.0.3
- gradle 4.4 (as defined in https://github.com/OB1Company/native-bazaar/blob/develop/android/gradle/wrapper/gradle-wrapper.properties)
- Java 1.8 (http://www.oracle.com/technetwork/java/javase/downloads/jdk8-downloads-2133151.html)

##### Using gradle to run app on-device

From the repository root, perform the following steps:

1. Ensure your device is available
```
$ adb devices
List of devices attached
8AAX0P48U       device
```
2. `cd ./android`
3. `./gradlew clean`
4. `./gradlew installRelease`
5. Find the application installed on your device.

### Contribution recommendations

Further development of Haven will be primarily sourced from community contributions. Below are some recommended entry points for contributors:

- More consistent 'basic' modularisation (e.g. add all the repetitive colors to `commonColors.js`
- Component modularisation to make code DRY
- Improve the folder structure, the module structure, and naming to have them make more sense (e.g. components are sometimes not put correctly to the sub-section of the Atomic Design)
- Upgrade modules to be up-to-date (e.g, `react-navigation` should be upgraded from 3 to 5)
- Will need to gradually transition to Functional Component leveraging React Hook
- Renaming price conversion functions (e.g. `minUnitAmountToBCH`) to something more generic

#### Unit testing

Some high priority targets for developing unit tests:

- Checkout flow, specifically the price calculation
- All the components that include prices
- The Order details screen for various order statuses 

#### End-to-end testing

Integration of an end-to-end testing tool such as Appium should be considered and applied to:

- The checkout flow
- The order flow for the various order statuses
- The send money flow
- The deep linking flow (just to make sure the various kinds of deep links open correct screens)

### Troubleshooting

#### `./gradlew clean` produces `java.lang.NullPointerException`

```
> Configure project :react-native-webview
FAILURE: Build failed with an exception.

* What went wrong:
A problem occurred configuring project ':react-native-webview'.
> java.lang.NullPointerException (no error message)
```

This typically occurs when the wrong Java version is used. The expected version for Java is 1.8. [See this StackOverflow question for help switching to 1.8](https://stackoverflow.com/questions/46513639/how-to-downgrade-java-from-9-to-8-on-a-macos-eclipse-is-not-running-with-java-9).

#### Missing modules

If you get an error like this:

```
While resolving module `react-native-vector-icons/Ionicons`, the Haste package `react-native-vector-icons` was found. However the module `Ionicons` could not be found within the package.
```

Type into the terminal: `rm ./node_modules/react-native/local-cli/core/__fixtures__/files/package.json`

### Contributors

We'd like to thank all of the contributors to the app throughout this journey, with a special mention to:

- Development
  - https://github.com/danZheng1993
  - https://github.com/heyradcode
  - https://github.com/reactular
  - https://github.com/hoffmabc 
- Design
  - https://github.com/morebrownies
  - https://github.com/miboediono
