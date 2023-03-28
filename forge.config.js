const path = require('path');
const packageJson = require('./package.json');

const { version } = packageJson;
const iconDir = path.resolve(__dirname, 'imgs');

module.exports = {
  packagerConfig: {
    asar: true,
    executableName: "mobazha-desktop",
    protocols: [
      {
        name: 'Mobazha',
        schemes: ['ob', 'mbz'],
      },
    ],
    icon: path.resolve(iconDir, process.platform === 'darwin' ? 'icon.icns' : 'icon.ico'),
    // ignore: 'mobazha',
    // extraResource: [
    //   'mobazha',
    // ],
    win32metadata: {
      ProductName: 'MobazhaDesktopClient',
      CompanyName: 'Mogaolei',
      FileDescription: 'Decentralized p2p marketplace for Cryptocurrencies',
      OriginalFilename: 'Mobazha',
    },
    osxNotarize: { // https://www.npmjs.com/package/electron-notarize#method-notarizeopts-promisevoid
      tool: 'notarytool',
      appleId: process.env.APPLE_ID,
      appleIdPassword: process.env.APPLE_APP_SPECIFIC_PASSWORD,
      teamId: '36RYSCJAD3',
    },
    // macOS code-signing configs. See https://www.electronjs.org/docs/latest/tutorial/code-signing#electron-forge
    osxSign: { // https://www.npmjs.com/package/electron-osx-sign#opts
      // identity: '...',
      hardenedRuntime: true,
      // entitlements: './static/entitlements.plist',
      // 'entitlements-inherit': './static/entitlements.plist',
      // keychain: 'build.keychain',
      ignore: 'Contents/Resources/app',
    },
  },
  rebuildConfig: {},
  makers: [
    {
      name: '@electron-forge/maker-squirrel',
      config: (arch) => ({ // https://js.electronforge.io/maker/squirrel/interfaces/makersquirrelconfig
        name: 'Mobazha',
        authors: 'Mogaolei',
        exe: 'Mobazha.exe',
        iconUrl: path.resolve(iconDir, 'icon.ico'),
        noMsi: true,
        setupExe: `Mobazha-${version}-${arch}-setup.exe`,
        setupIcon: path.resolve(iconDir, 'icon.ico'),
        certificateFile: path.resolve(__dirname, '.travis', 'mobazha.org.pfx'),
        certificatePassword: process.env.PFX_PASSWORD,
      }),
    },
    {
      name: '@electron-forge/maker-dmg',
      config: (arch) => ({ // https://js.electronforge.io/maker/dmg/interfaces/makerdmgconfig
        background: path.resolve(iconDir, 'osx-finder_background.png'),
        format: 'ULFO',
        icon: path.resolve(iconDir, 'icon.icns'),
        name: `Mobazha-${version}-${arch}`,
        overwrite: true,
      }),
    },
    {
      name: '@electron-forge/maker-deb',
      config: {},
    },
    {
      name: '@electron-forge/maker-rpm',
      config: {},
    },
  ],
  hooks: {
    generateAssets: async (platform, arch) => {
      console.info('Packages built at:', platform, arch);
    },
    prePackage: async (platform, arch) => {
      console.info('Packages built at:', platform, arch);
    },
    postPackage: async (forgeConfig, options) => {
      console.info('Packages built at:', options.outputPaths);
    },
  },
};
