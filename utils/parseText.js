import { Linking } from 'react-native';
import { UNIVERSAL_LINK_PREFIX } from './navigation';

export const WWW_URL_PATTERN = /^www\./i;

// reference: https://stackoverflow.com/questions/48874571/what-does-the-new-bch-regex-mean
const BITCOINCASH_REGEX = /\b((bitcoincash:)?(q|p)[a-z0-9]{41})|^((BITCOINCASH:)?(Q|P)[A-Z0-9]{41})$/gm;

// reference: https://stackoverflow.com/questions/21683680/regex-to-match-bitcoin-addresses
const BITCOIN_REGEX = /\b(bitcoin:)?[13][a-km-zA-HJ-NP-Z1-9]{25,34}$/gm;

// reference: https://www.reddit.com/r/zec/comments/8mxj6x/simple_regex_to_validate_a_zcash_tz_address/
const ZCASH_REGEX = /\bt1[a-zA-Z0-9]{33}$/gm;

// reference: https://stackoverflow.com/questions/23570080/how-to-determine-if-litecoin-address-is-valid
const LITECOIN_REGEX = /\b[LM3][a-km-zA-HJ-NP-Z1-9]{26,33}$/gm;

const ETHER_REGEX = /\b0x[a-fA-F0-9]{40}$/gm;

const CONFLUX_REGEX = /^cfx.*$/gm;

const HASH_TAG_REGEX = /#(\w+)/;

const handleUrlPress = onDeeplinkPress => (url) => {
  if (url.startsWith(UNIVERSAL_LINK_PREFIX)) {
    onDeeplinkPress(url);
  } else {
    const processedUrl = WWW_URL_PATTERN.test(url) ? `http://${url}` : url;
    Linking.canOpenURL(processedUrl).then((supported) => {
      if (supported) {
        Linking.openURL(processedUrl);
      }
    });
  }
};

export const getOBParsePatterns = (linkStyle, onDeeplinkPress, onCrypoLink, onHashtag) => [
  { type: 'url', style: linkStyle, onPress: handleUrlPress(onDeeplinkPress) },
  {
    pattern: /\bob:\/\/((\w+)*\/)([\w\-\.]+[^#?\s]+)(.*)?(#[\w\-]+)?/igm,
    style: linkStyle,
    onPress: onDeeplinkPress,
  },
  {
    pattern: BITCOINCASH_REGEX,
    style: linkStyle,
    onPress: onCrypoLink('BCH'),
  },
  {
    pattern: BITCOIN_REGEX,
    style: linkStyle,
    onPress: onCrypoLink('BTC'),
  },
  {
    pattern: ZCASH_REGEX,
    style: linkStyle,
    onPress: onCrypoLink('ZEC'),
  },
  {
    pattern: LITECOIN_REGEX,
    style: linkStyle,
    onPress: onCrypoLink('LTC'),
  },
  {
    pattern: ETHER_REGEX,
    style: linkStyle,
    onPress: onCrypoLink('ETH'),
  },
  {
    pattern: CONFLUX_REGEX,
    style: linkStyle,
    onPress: onCrypoLink('CFX'),
  },
  {
    pattern: HASH_TAG_REGEX,
    style: linkStyle,
    onPress: onHashtag,
  },
];
