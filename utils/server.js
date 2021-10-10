import base64 from 'base-64';
import RnFs from 'react-native-fs';

class ServerConfig {
  serverToken = '';
  username='';
  password='';
  constructor() {
    this.username = 'username';
    this.password = 'password';
  }
  setServerToken(serverToken) {
    this.serverToken = serverToken;
  }
  getServerToken() {
    return this.serverToken;
  }
  getAuthToken() {
    return `Basic ${base64.encode(`${this.username}:${this.password}`)}`;
  }
  getAuthHeader(username = this.username, password = this.password) {
    return {
      authorization: `Basic ${base64.encode(`${username}:${password}`)}`,
      cookie: `OpenBazaar_Auth_Cookie=${this.serverToken}`,
      Accept: 'application/json',
      'Content-Type': 'application/json'
    };
  }
}

export const serverConfig = new ServerConfig();

export const SERVER_PATH = `${RnFs.DocumentDirectoryPath}/Mobazha`;
export const SERVER_UNZIP_PATH = `${RnFs.DocumentDirectoryPath}/Mobazha_Unzip`;
