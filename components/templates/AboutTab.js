import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback, Clipboard, Linking } from 'react-native';
import * as striptags from 'striptags';
import QRCode from 'react-native-qrcode-svg';
import * as _ from 'lodash';
import he from 'he';

import { primaryTextColor, secondaryTextColor, linkTextColor, borderColor } from '../commonColors';
import { createStoreUrlFromPeerID } from '../../utils/navigation';
import { eatSpaces } from '../../utils/string';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'white',
    paddingTop: 20,
    paddingBottom: 20,
    paddingHorizontal: 15,
    borderTopWidth: 0,
  },
  qrcode: {
    alignItems: 'center',
  },
  contactInfoWrapper: {
    paddingTop: 20,
    borderBottomWidth: 1,
    borderColor,
  },
  descriptionWrapper: {
    paddingVertical: 10,
  },
  description: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  contactInfo: {
    flexDirection: 'row',
    marginTop: 20,
  },
  contactInfoLabel: {
    width: 70,
    height: 16,
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
    alignItems: 'center',
  },
  contactInfoContent: {
    minHeight: 20,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: linkTextColor,
  },
  contactLinkImage: {
    width: 15,
    height: 15,
  },
  storeAddress: {
    fontSize: 11,
    paddingTop: 10,
    fontWeight: 'normal',
    fontStyle: 'normal',
    textAlign: 'center',
    letterSpacing: 0,
    color: primaryTextColor,
    marginVertical: 4,
  },
  doCopy: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: linkTextColor,
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 200,
    top: 100,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

const capitalizeFirstLetter = string => string.charAt(0).toUpperCase() + string.slice(1);

class AboutTab extends PureComponent {
  state = {
    copied: false,
  };

  handleCopyLink = () => {
    const { peerID } = this.props.profile;
    Clipboard.setString(createStoreUrlFromPeerID(peerID));

    this.setState({ copied: true });
    setTimeout(() => this.setState({ copied: false }), 2000);
  };

  handleOpenURL = (url) => {
    Linking.canOpenURL(url)
      .then((supported) => {
        if (!supported) {
          console.log(`Can't handle url: ${url}`);
        } else {
          return Linking.openURL(url);
        }
      })
      .catch((err) => {
        console.error('An error occurred', err);
      });
  };

  handlePressContact = (type, contact) => {
    switch (type) {
      case 'website':
        {
          let url = contact.trim();
          const regex = /^[a-zA-Z]+:\/\//;
          if (!url.match(regex)) {
            url = `http://${url}`;
          }
          this.handleOpenURL(url);
        }
        break;
      case 'phoneNumber':
        this.handleOpenURL(`tel:${contact}`);
        break;
      case 'email':
        this.handleOpenURL(`mailto:${contact}?cc=?subject=''?body=''`);
        break;
      default:
        break;
    }
  };

  renderContactInfo = contactInfo =>
    Object.keys(contactInfo).map((e, idx) =>
      typeof contactInfo[e] === 'string' &&
        !_.isEmpty(contactInfo[e]) && (
          <View style={styles.contactInfo} key={idx}>
            <Text style={styles.contactInfoLabel}>
              {e === 'phoneNumber' ? I18n.t('components.templates.AboutTab.phone') : capitalizeFirstLetter(e)}
            </Text>
            <TouchableWithoutFeedback onPress={() => this.handlePressContact(e, contactInfo[e])}>
              <View style={{ flex: 1 }}>
                <Text style={styles.contactInfoContent} numberOfLines={3}>
                  {contactInfo[e]}
                  &nbsp;
                </Text>
              </View>
            </TouchableWithoutFeedback>
          </View>
      ));

  render() {
    const { profile } = this.props;
    const { copied } = this.state;
    const { contactInfo, about, peerID } = profile;
    return (
      <View style={styles.wrapper}>
        <View style={styles.qrcode}>
          <TouchableWithoutFeedback onPress={this.handleCopyLink}>
            <View>
              <QRCode value={createStoreUrlFromPeerID(peerID)} size={200} />
            </View>
          </TouchableWithoutFeedback>
          <Text style={styles.storeAddress}>{peerID}</Text>
          <TouchableWithoutFeedback onPress={this.handleCopyLink}>
            <View>
              <Text style={styles.doCopy}>{I18n.t('components.templates.AboutTab.copy')}</Text>
            </View>
          </TouchableWithoutFeedback>
        </View>
        <View style={about && styles.contactInfoWrapper}>
          {contactInfo && this.renderContactInfo(contactInfo)}
        </View>
        <View style={about && styles.descriptionWrapper}>
          <Text style={styles.description}>
            {about && eatSpaces(he.decode(striptags.default(about)))}
          </Text>
        </View>
        {copied && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{I18n.t('components.templates.AboutTab.link_copied')}</Text>
          </View>
        )}
      </View>
    );
  }
}

export default AboutTab;
