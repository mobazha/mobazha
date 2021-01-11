import React from 'react';
import { Clipboard, View, Text, Platform, StyleSheet } from 'react-native';
import { WebView } from 'react-native-webview';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import NavCloseButton from '../atoms/NavCloseButton';
import Button from '../atoms/FullButton';
import Header from '../molecules/Header';
import { OBLightModal } from '../templates/OBModal';

import {I18n} from '../../langs/I18n';

const styles = {
  buttonWrapper: {
    marginBottom: ifIphoneX(44, 10),
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 120,
    bottom: 150,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  activityIndicator: {
    ...StyleSheet.absoluteFill,
    alignItems: 'center',
    justifyContent: 'center',
  },
  webviewWrapper: {
    paddingHorizontal: 6,
    flex: 1,
  },
};

export default class ContractModal extends React.Component {
  state = {
    copied: false,
    showSpinner: true,
  }

  handleCopy = () => {
    const { orderDetails } = this.props;
    Clipboard.setString(JSON.stringify(orderDetails, null, 2));

    this.setState({ copied: true });
    setTimeout(() => this.setState({ copied: false }), 2000);
  }

  render() {
    const { show, onClose, orderDetails } = this.props;
    const { copied } = this.state;

    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={show}
        onRequestClose={onClose}
      >
        <Header
          left={<NavCloseButton />}
          modal
          onLeft={onClose}
          title={I18n.t('components.templates.ContractModal.view_contract')}
        />
        <View style={styles.webviewWrapper}>
          <WebView
            source={{ html: `<div><pre>${JSON.stringify(orderDetails, null, 2)}</pre></div>` }}
            scalesPageToFit={Platform.OS === 'android'}
            useWebKit={false}
          />
        </View>
        <Button
          title="Copy to clipboard"
          wrapperStyle={styles.buttonWrapper}
          onPress={this.handleCopy}
          style={styles.firstButton}
        />
        {copied && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{I18n.t('components.templates.ContractModal.copied')} </Text>
          </View>
        )}
      </OBLightModal>
    );
  }
}
