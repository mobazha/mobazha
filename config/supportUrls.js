import { Platform } from 'react-native';
import { version } from '../package.json';

export const FAQ_URL = 'https://gethaven.app/faq';
export const TELEGRAM_URL = 'https://t.me/joinchat/kLeb3weo3pk4ZWY1';
export const EMAIL_URL = `mailto:admin@mobazha.com?subject=Mobazha%20Customer%20Support&body=%0A%0ABuild%20version:%20${version}%0AOS:%20${Platform.OS}`;
