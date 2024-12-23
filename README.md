# Mobazha Client v3

This is the reference client for the Mobazha network. It is an interface for your Mobazha node, to use it you will need to run an [Mobazha node](https://github.com/mobazha/mobazha-go) either locally or on a remote server.

For full installable versions of the Mobazha app, with the server and client bundled together, go to [the Mobazha download page.](https://www.mobazha.org/download/)

[![Build Status](https://travis-ci.org/Mobazha/mobazha-desktop.svg?branch=master)](https://travis-ci.org/Mobazha/mobazha-desktop)

## Getting Started

To create a local development copy of the reference client, clone the client repository into a directory of your choice:
- `git clone https://github.com/mobazha/mobazha-desktop`

Make sure you have Node.js and NPM installed. Node versions older than 20.18.1 or NPM versions older than 10.8.2 may not work.


### Installation

1. Navigate to the directory you cloned the repo into.
2. Enter `npm install`

### Running


### Linting

`npm run lint` will run eslint on the JS files.

`npm run lint:watch` will run eslint on any JS file changes.

### Testing

`npm run test` will execute test files in the test folder.

`npm run test:watch` will execute the tests on any file changes.


## Built With

* [Electron](https://electron.atom.io/)
* [Vue](https://vuejs.org/)
* [Backbone](http://backbonejs.org/)

## Contributing

We welcome contributions to the reference client. The best way to get started is to look for an issue with the [Help Wanted label](https://github.com/Mobazha/mobazha-desktop/labels/help%20wanted).

You can also look for issues with the [bug label](https://github.com/Mobazha/mobazha-desktop/labels/bug). These are confirmed bugs that need to be fixed.

Contributions are expected to match the coding style already present in this repo, and must pass es-lint with no errors.

Contributions that make visual changes are also expected to match the repo's current style.

If you want to help with translations, please request to join the translation team at [https://www.transifex.com/ob1/mobazha](https://www.transifex.com/ob1/mobazha).

You can request new languages there, and contribute to the translation of existing languages.

New languages are usually added when they reach 80% or more completion, and not removed from the client unless they fall below 60% for several releases.

## License
This project is licensed under the MIT License. You can view [LICENSE.MD](https://github.com/mobazha/mobazha-desktop/blob/master/LICENSE) for more details.

