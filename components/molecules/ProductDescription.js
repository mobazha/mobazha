import React, { PureComponent } from 'react';
import { Text, TouchableWithoutFeedback, Platform, ActivityIndicator, View, StyleSheet } from 'react-native';
import * as striptags from 'striptags';
import he from 'he';
import { WebView } from 'react-native-webview';
import * as _ from 'lodash';

import { primaryTextColor, foregroundColor } from '../commonColors';
import ProductSection from '../atoms/ProductSection';
import Header from './Header';
import NavCloseButton from '../atoms/NavCloseButton';

import { OBLightModal } from '../templates/OBModal';

import { eatSpaces } from '../../utils/string';
import { cssCode } from '../../utils/webview';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  content: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  showAllButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    marginTop: 12,
  },
  showAllText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  activityIndicator: {
    ...StyleSheet.absoluteFill,
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptySection: {
    paddingBottom: 0,
  },
  emptyText: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'italic',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#8A8A8F',
  },
  webviewWrapper: {
    paddingHorizontal: 6,
    flex: 1,
  },
};

export default class ProductDescription extends PureComponent {
  state = {
    showModal: false,
    showSpinner: true,
  };

  getHtmlContent = () => {
    const { description } = this.props;
    if (description !== he.decode(striptags.default(description))) {
      return description;
    } else {
      const parts = description.split('\n');
      const results = parts.map(part => `<p>${part}</p>`);
      return results.join('');
    }
  }

  handleShowModal = () => {
    eventTracker.trackEvent('ListingSectionTapped-ReadMore');
    this.setState({ showModal: true });
  };

  handleHideModal = () => {
    this.setState({ showModal: false });
  };

  handleHideSpinner = () => {
    this.setState({ showSpinner: false });
  }

  render() {
    const { description } = this.props;
    const { showModal, showSpinner } = this.state;

    if (_.isEmpty(description)) {
      return (
        <ProductSection>
          <Text style={styles.emptyText}>{I18n.t('components.molecules.ProductDescription.empty_text')}</Text>
        </ProductSection>
      );
    }

    return (
      <ProductSection>
        <Text style={styles.content} numberOfLines={3}>
          {eatSpaces(he.decode(striptags.default(description)))}
        </Text>
        <TouchableWithoutFeedback onPress={this.handleShowModal}>
          <View style={styles.showAllButton}>
            <Text style={styles.showAllText}>{I18n.t('components.molecules.ProductDescription.read_more')}</Text>
          </View>
        </TouchableWithoutFeedback>
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={this.handleHideModal}
        >
          <Header modal left={<NavCloseButton />} onLeft={this.handleHideModal} />
          <View style={styles.webviewWrapper}>
            <WebView
              onLoadStart={this.handleHideSpinner}
              onError={this.handleHideSpinner}
              originWhitelist={['*']}
              source={{
                html: `${cssCode} ${this.getHtmlContent()}`,
              }}
              scalesPageToFit={false}
              automaticallyAdjustContentInsets
              useWebKit={false}
            // injectedJavaScript={jsCode}
            // javaScriptEnabled
            />
          </View>
          {showSpinner && (
            <View style={styles.activityIndicator}>
              <ActivityIndicator size="large" color="#8a8a8f" />
            </View>
          )}
        </OBLightModal>
      </ProductSection>
    );
  }
}
