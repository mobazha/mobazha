# Mobazha Client v3

This is the reference client for the Mobazha network. It is an interface for your Mobazha node, to use it you will need to run an [Mobazha node](https://github.com/mobazha/mobazha-go) either locally or on a remote server.

For full installable versions of the Mobazha app, with the server and client bundled together, go to [the Mobazha download page.](https://www.mobazha.org/download/)

[![Build Status](https://github.com/mobazha/mobazha-desktop/actions/workflows/build.yml/badge.svg)](https://github.com/mobazha/mobazha-desktop/actions)

## Getting Started

To create a local development copy of the reference client, clone the client repository into a directory of your choice:
- `git clone https://github.com/mobazha/mobazha-desktop`

Make sure you have Node.js and NPM installed. Node versions older than 20.18.1 or NPM versions older than 10.8.2 may not work.

## Preparation

Download the Mobazha node for your current OS version from latest [Mobazha node release](https://github.com/mobazha/mobazha-go/releases) and place it in the `build/extraResources/mobazha` subdirectory. The node should be named `mobazha.exe` for Windows or `mobazhad` for macOS/Linux.

### Running

1. Navigate to the directory you cloned the repo into.
2. In the frontend subfolder, enter `npm install`.
3. In the root subfolder, enter `npm install`.
4. In the root folder, run `npm run dev`

## Building

To build the application for different platforms, use the following commands:

- For Windows: `npm run build-w`
- For macOS: `npm run build-m`
- For Linux: `npm run build-l`

## Built With

* [Electron](https://electron.atom.io/)
* [Vue](https://vuejs.org/)
* [Backbone](http://backbonejs.org/)

## Demo

### Bitcoin
[![Bitcoin](https://img.youtube.com/vi/WSvLQRjqdX4/0.jpg)](https://www.youtube.com/watch?v=WSvLQRjqdX4)

### Bitcoin Cash
[![Bitcoin Cash](https://img.youtube.com/vi/Abo7YJYJgrE/0.jpg)](https://www.youtube.com/watch?v=Abo7YJYJgrE)

### ZCash
[![ZCash](https://img.youtube.com/vi/dWXfLLdqUEI/0.jpg)](https://www.youtube.com/watch?v=dWXfLLdqUEI)

### Litecoin
[![Litecoin](https://img.youtube.com/vi/9do7zQ8ey90/0.jpg)](https://www.youtube.com/watch?v=9do7zQ8ey90)

### Web and Moderation
[![Web and Moderation](https://img.youtube.com/vi/_PUywMOV8nk/0.jpg)](https://www.youtube.com/watch?v=_PUywMOV8nk)

### Remote Store Setup
[![Remote Store Setup](https://img.youtube.com/vi/i84R35SCkUA/0.jpg)](https://www.youtube.com/watch?v=i84R35SCkUA)

## Contributing

We welcome contributions to the reference client. The best way to get started is to look for an issue with the [Help Wanted label](https://github.com/mobazha/mobazha-desktop/labels/help%20wanted).

You can also look for issues with the [bug label](https://github.com/mobazha/mobazha-desktop/labels/bug).

Contributions are expected to match the coding style already present in this repo, and must pass es-lint with no errors.

Contributions that make visual changes are also expected to match the repo's current style.

## License
This project is licensed under the GPLv3 License. You can view [LICENSE.MD](https://github.com/mobazha/mobazha/blob/main/LICENSE) for more details.