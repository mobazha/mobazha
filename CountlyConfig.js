import Config from 'react-native-config';
import Countly from 'countly-sdk-react-native-bridge';

export const COUNTLY_ROOT_URL = Config.COUNTLY_ROOT_URL;
export const COUNTLY_APP_KEY = Config.COUNTLY_APP_KEY;

Countly.isDebug = __DEV__;
