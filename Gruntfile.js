const path = require('path');

module.exports = function (grunt) {
  // Project configuration.
  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    'electron-installer-debian': {
      options: {
        productName: 'Mobazha',
        name: 'mobazha',
        arch: 'amd64',
        version: '3.0.0',
        bin: 'mobazha',
        maintainer: 'Mobazha <support@mobazha.com>',
        rename(dest) {
          return `${dest}<%= name %>_<%= version %>_<%= arch %>.deb`;
        },
        productDescription: 'Decentralized Peer to Peer Marketplace for Bitcoin',
        lintianOverrides: [
          'changelog-file-missing-in-native-package',
          'executable-not-elf-or-script',
          'extra-license-file',
        ],
        icon: 'imgs/icon.png',
        categories: ['Utility'],
        priority: 'optional',
      },
      'app-with-asar': {
        src: 'temp-linux64/openbazaar-linux-x64/',
        dest: 'build-linux64/',
        rename(dest) {
          return path.join(dest, '<%= name %>_<%= arch %>.deb');
        },
      },
    },
    'create-windows-installer': {
      x32: {
        appDirectory: grunt.option('appdir'),
        outputDirectory: grunt.option('outdir'),
        name: grunt.option('appname'),
        productName: grunt.option('appname'),
        authors: 'Mobazha',
        owners: 'Mobazha',
        exe: `${grunt.option('appname')}.exe`,
        description: `${grunt.option('appname')}`,
        version: grunt.option('obversion') || '',
        title: 'Mobazha',
        iconUrl: 'http://openbazaar.org/assets/windows-icon.ico',
        setupIcon: 'imgs/windows-icon.ico',
        skipUpdateIcon: true,
        loadingGif: 'imgs/windows-loading.gif',
        noMsi: true,
      },
    },
  });
  grunt.loadNpmTasks('grunt-electron-installer-debian');
  grunt.loadNpmTasks('grunt-electron-installer');
  // Default task(s).
  grunt.registerTask('default', ['electron-installer-debian']);
};
