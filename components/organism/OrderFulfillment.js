import React, { PureComponent } from 'react';
import { Alert, Clipboard, View, Text } from 'react-native';
import { get, isEmpty } from 'lodash';
import decode from 'unescape';

import InputGroup from '../atoms/InputGroup';
import StaticField from '../atoms/StaticField';

import { primaryTextColor, borderColor } from '../commonColors';
import { timeSince } from '../../utils/time';

import {I18n} from '../../langs/I18n';

const styles = {
  fulfillWrapper: {
    paddingVertical: 12,
  },
  memoContainer: {
    paddingLeft: 9,
    borderLeftWidth: 2,
    borderColor,
    marginTop: 13,
    marginBottom: 8,
  },
  description: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 20,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 240,
    top: 0,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  timestamp: {
    color: '#9b9b9b',
  },
};

class OrderFulfillment extends PureComponent {
  state = {
    copied: false,
  };

  handleCopy = () => {
    const { detail } = this.props;
    const { trackingNumber } = get(detail, 'physicalDelivery[0]', {});
    if (isEmpty(trackingNumber)) {
      Alert.alert('No tracking number to copy!');
      return;
    }

    Clipboard.setString(trackingNumber);
    this.setState({ copied: true });
    setTimeout(() => this.setState({ copied: false }), 2000);
  }

  renderDetail() {
    const { detail } = this.props;
    const physical = get(detail, 'physicalDelivery[0]');
    const digital = get(detail, 'digitalDelivery[0]');

    const { copied } = this.state;

    if (!isEmpty(physical)) {
      return (
        <View style={styles.detailWrapper}>
          <StaticField label="Shipping via" value={physical.shipper} />
          <StaticField
            label="Tracking #"
            value={physical.trackingNumber}
            onCopy={this.handleCopy}
          />
          {copied && (
            <View style={styles.overlay}>
              <Text style={styles.overlayText}>{I18n.t('components.organism.OrderFulfillment.tracking_number_copied')}</Text>
            </View>
          )}
        </View>
      );
    } else if (!isEmpty(digital)) {
      return (
        <View style={styles.detailWrapper}>
          <StaticField label={I18n.t('components.organism.OrderFulfillment.file_url')} value={digital.url} isLink />
          <StaticField label={I18n.t('components.organism.OrderFulfillment.password')} value={digital.password} />
        </View>
      );
    }

    return (
      <Text style={styles.description}>
        {I18n.t('components.organism.OrderFulfillment.fulfilled_info')}
      </Text>
    );
  }

  render() {
    const { detail } = this.props;
    return (
      <InputGroup
        title={I18n.t('components.organism.OrderFulfillment.order_fulfilled')}
        actionTitle={timeSince(new Date(detail.timestamp))}
        actionStyle={styles.timestamp}
      >
        <View style={styles.fulfillWrapper}>
          {this.renderDetail()}
          {!isEmpty(detail.note) && (
            <View style={styles.memoContainer}>
              <Text style={styles.description}>
                {decode(detail.note)}
              </Text>
            </View>
          )}
        </View>
      </InputGroup>
    );
  }
}

export default OrderFulfillment;
