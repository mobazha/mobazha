import React from 'react';
import { View, ScrollView, Image, Text, Linking } from 'react-native';
import Hyperlink from 'react-native-hyperlink';

import Button from '../atoms/FullButton';
import { OBLightModal } from '../templates/OBModal';

import { onboardingStyles, footerStyles } from '../../utils/styles';
import { brandColor } from '../commonColors';
import { EMAIL_URL } from '../../config/supportUrls';

import {I18n} from '../../langs/I18n';

const shieldImg = require('../../assets/images/privacyShield.png');
const bottomImg = require('../../assets/images/privacyBottom.png');

const styles = {
  contentWrapper: {
    flex: 1,
  },
  bold: {
    fontWeight: 'bold',
  },
  buttonWrapper: {
    marginBottom: 10,
  },
  header: {
    height: 96,
    width: '100%',
    alignItems: 'center',
    justifyContent: 'center',
  },
  logo: {
    width: 47,
    height: 52,
  },
  privacyText: {
    fontSize: 13,
    lineHeight: 13,
    color: 'white',
    backgroundColor: '#8cd885',
    left: 16,
    paddingLeft: 11,
    paddingRight: 9,
    paddingTop: 7,
    paddingBottom: 5,
    letterSpacing: 1,
    borderRadius: 13,
    overflow: 'hidden',
    alignSelf: 'flex-start',
    marginBottom: 12,
  },
  privacyDescriptionNormal: {
    marginTop: 16,
    fontSize: 16,
    // lineHeight: 26,
    color: '#404040',
  },
  privacyDescriptionBold: {
    marginTop: 16,
    fontSize: 16,
    // lineHeight: 26,
    color: '#404040',
    fontWeight: 'bold',
  },
  hyperlinkContainer: {
    paddingHorizontal: 16,
    // flex: 1,
  },
  privacyButtonText: {
    color: brandColor,
    textDecorationLine: 'underline',
  },
  bottomImg: {
    position: 'absolute',
    bottom: 0,
    left: 0,
  },
};

const scrollStyleProps = {
  style: {
    flex: 1,
  },
  // showsVerticalScrollIndicator: false,
};

export default class EULAModal extends React.Component {
  handleShowModal = (url) => {
    if (url === 'mailto:admin@mobazha.com') {
      Linking.openURL(EMAIL_URL);
    }
  };

  render() {
    const { show, onClose } = this.props;
    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={show}
      >
        <Image style={styles.bottomImg} source={bottomImg} resizeMode="contain" />
        <View style={styles.header}>
          <Image style={styles.logo} source={shieldImg} resizeMode="contain" />
        </View>
        <Text style={[styles.privacyText]}>
        {I18n.t('components.templates.EULAModal.eula')}          
        </Text>
        <ScrollView {...scrollStyleProps}>
          <Hyperlink
            style={styles.hyperlinkContainer}
            linkStyle={styles.privacyButtonText}
            onPress={this.handleShowModal}
          >
            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description1')}              
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description2')} 
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description3')} 
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description4')}             
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description5')}               
            </Text>

            {/* <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description6')} 
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description7')} 
            </Text> */}

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description8')}  
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description9')}               
            </Text>

            {/* <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description10')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description11')} 
            </Text> */}

            {/* <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description12')} 
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description13')}              
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description14')}
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description15')}
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description16')}
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description17')}              
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description18')}              
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description19')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description20')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description21')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description22')} 
              
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description23')} 
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description24')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description25')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description26')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description27')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description28')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description29')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description30')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description31')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description32')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description33')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
              {I18n.t('components.templates.EULAModal.privacy_description34')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
              {I18n.t('components.templates.EULAModal.privacy_description35')}               
            </Text>

            <Text style={styles.privacyDescriptionBold}>
            {I18n.t('components.templates.EULAModal.privacy_description36')}               
            </Text>
            <Text style={styles.privacyDescriptionNormal} textBreakStrategy="simple">
            {I18n.t('components.templates.EULAModal.privacy_description37')}               
            </Text> */}

          </Hyperlink>
        </ScrollView>
        <View style={footerStyles.roundButtonContainer}>
          <Button
            wrapperStyle={onboardingStyles.button}
            title={I18n.t('components.templates.EULAModal.iaccept')}
            onPress={onClose}
          />
        </View>
      </OBLightModal>
    );
  }
}
