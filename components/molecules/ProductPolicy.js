import React, { PureComponent } from 'react';
import { Text, Platform, ActivityIndicator, View, StyleSheet } from 'react-native';
import * as _ from 'lodash';
import { WebView } from 'react-native-webview';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { primaryTextColor } from '../commonColors';
import ProductSection from '../atoms/ProductSection';
import Header from './Header';
import NavCloseButton from '../atoms/NavCloseButton';
import OptionGroup from '../atoms/OptionGroup';
import { OBLightModal } from '../templates/OBModal';
import { cssCode } from '../../utils/webview';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  title: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    lineHeight: 15,
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
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
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 185,
  },
  emptyIcon: {
    marginBottom: 10,
  },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    width: 294,
    textAlign: 'center',
  },
};

export default class ProductPolicy extends PureComponent {
  state = {
    showModal: false,
    showSpinner: true,
  };

  handleShowModal = () => {
    const { policy } = this.props;
    eventTracker.trackEvent(`ListingSectionTapped-${policy}`);
    this.setState({ showModal: true });
  };

  handleHideModal = () => {
    this.setState({ showModal: false });
  };

  handleHideSpinner = () => {
    this.setState({ showSpinner: false });
  }

  renderEmptyContent = policy => (
    <View style={styles.emptyWrapper}>
      <Ionicons style={styles.emptyIcon} name="ios-document" size={50} color="#8a8a8f" />
      <Text style={styles.emptyText}>
        {I18n.t('components.molecules.ProductPolicy.no_provided', { policy: policy })}
      </Text>
    </View>
  );

  render() {
    const { policy, content } = this.props;
    const { showModal, showSpinner } = this.state;
    return (
      <ProductSection>
        <OptionGroup onPress={this.handleShowModal} noBorder>
          <Text style={styles.title}>{policy}</Text>
        </OptionGroup>
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={this.handleHideModal}
        >
          <Header modal left={<NavCloseButton />} onLeft={this.handleHideModal} />
          {_.isEmpty(content) ? (
            this.renderEmptyContent(policy)
          ) : (
              <View style={styles.webviewWrapper}>
                <WebView
                  onLoadStart={this.handleHideSpinner}
                  onError={this.handleHideSpinner}
                  originWhitelist={['*']}
                  source={{
                    html: `${cssCode} ${content}`,
                    baseUrl: '',
                  }}
                  // injectedJavaScript={jsCode}
                  // javaScriptEnabled
                  scalesPageToFit={Platform.OS === 'android'}
                  useWebKit={false}
                />
              </View>
            )}
          {!_.isEmpty(content) && showSpinner && (
            <View style={styles.activityIndicator}>
              <ActivityIndicator size="large" color="#8a8a8f" />
            </View>
          )}
        </OBLightModal>
      </ProductSection>
    );
  }
}
