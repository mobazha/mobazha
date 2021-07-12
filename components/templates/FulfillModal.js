import React from 'react';
import { View, ActivityIndicator } from 'react-native';
import * as _ from 'lodash';
import { connect } from 'react-redux';

import { primaryTextColor, linkTextColor, brandColor } from '../commonColors';

import { fulfillOrder } from '../../reducers/order';

import { OBLightModal } from '../templates/OBModal';
import { chatStyles } from '../../utils/styles';
import NavCloseButton from '../atoms/NavCloseButton';
import TextInput from '../atoms/TextInput';
import LinkText from '../atoms/LinkText';
import Header from '../molecules/Header';

import {I18n} from '../../langs/I18n';

const MAJOR_PADDING = chatStyles.avatarImage.marginLeft;

const styles = {
  contentContainer: {
    paddingHorizontal: 17,
  },
  input: {
    flex: 1,
    width: '100%',
    fontSize: 15,
    fontWeight: '400',
    fontStyle: 'normal',
    color: primaryTextColor,
    backgroundColor: 'white',
    textAlignVertical: 'top',
    paddingTop: 7,
  },
  placeholder: {
    fontStyle: 'italic',
    fontSize: 15,
    paddingTop: 7,
    lineHeight: 20,
  },
  indicator: {
    marginRight: 32,
  },
};

class FulfillModal extends React.PureComponent {
  state = {
    processing: false,
    shipper: '',
    trackingNumber: '',
    fileUrl: '',
    filePassword: '',
    note: '',
  }

  handleSuccess = () => {
    this.setState({ processing: false });
    this.props.onHideFulfill();
  }

  handleFailure = () => {
    setTimeout(() => {
      this.setState({ processing: false });
    }, 5000);
  }

  handleFulfill = () => {
    const {
      fulfillOrder, orderId, contractType, buyerId,
    } = this.props;
    const {
      shipper, trackingNumber, fileUrl, filePassword, note,
    } = this.state;
    let requestBody = {};
    switch (contractType) {
      case 'SERVICE':
        requestBody = { orderId, note };
        break;
      case 'PHYSICAL_GOOD':
        requestBody = {
          orderId,
          physicalDelivery: [{ shipper, trackingNumber }],
          note,
        };
        break;
      default:
        requestBody = {
          orderId,
          digitalDelivery: [{ url: fileUrl, password: filePassword }],
          note,
        };
        break;
    }
    this.setState({ processing: true });
    fulfillOrder({ body: requestBody, peerID: buyerId, onSuccess: this.handleSuccess, onFailure: this.handleFailure });
  }

  updateState = field => (value) => {
    const dict = {};
    dict[field] = value;
    this.setState(dict);
  }

  renderContent = () => {
    const { contractType } = this.props;
    const {
      shipper, trackingNumber, fileUrl, filePassword, note,
    } = this.state;
    if (contractType === 'PHYSICAL_GOOD') {
      return (
        <View style={styles.contentContainer}>
          <TextInput
            title={I18n.t('components.templates.FulfillModal.shipping_carrier')}
            placeholder={I18n.t('components.templates.FulfillModal.carrier_hint')}
            value={shipper}
            onChangeText={this.updateState('shipper')}
          />
          <TextInput
            title={I18n.t('components.templates.FulfillModal.tracking_number')}
            placeholder={I18n.t('components.templates.FulfillModal.tracking_number_hint')}
            value={trackingNumber}
            onChangeText={this.updateState('trackingNumber')}
          />
          <TextInput
            title={I18n.t('components.templates.FulfillModal.note')}
            placeholder={I18n.t('components.templates.FulfillModal.note_hint')}
            value={note}
            onChangeText={this.updateState('note')}
            noBorder
          />
        </View>
      );
    }

    if (contractType === 'DIGITAL_GOOD') {
      return (
        <View style={styles.contentContainer}>
          <TextInput
            title={I18n.t('components.templates.FulfillModal.file_url')}
            placeholder={I18n.t('components.templates.FulfillModal.file_url_hint')}
            value={fileUrl}
            onChangeText={this.updateState('fileUrl')}
          />
          <TextInput
            title={I18n.t('components.templates.FulfillModal.password')}
            placeholder={I18n.t('components.templates.FulfillModal.password_hint')}
            value={filePassword}
            onChangeText={this.updateState('filePassword')}
          />
          <TextInput
            title={I18n.t('components.templates.FulfillModal.note')}
            placeholder={I18n.t('components.templates.FulfillModal.note_hint')}
            value={note}
            onChangeText={this.updateState('note')}
            noBorder
          />
        </View>
      );
    }

    return (
      <View style={styles.contentContainer}>
        <TextInput
          noTitle
          placeholder={I18n.t('components.templates.FulfillModal.add_a_note')}
          value={note}
          onChangeText={this.updateState('note')}
          noBorder
        />
      </View>
    );
  }

  render() {
    const { isOpen, onHideFulfill } = this.props;
    const { processing } = this.state;
    const headerRightProps = processing ? {
      right: <ActivityIndicator style={styles.indicator} color={brandColor} size="small" />,
    } : {
      right: <LinkText text={I18n.t('components.templates.FulfillModal.done')} color={linkTextColor} />,
      onRight: this.handleFulfill,
    };
    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={isOpen}
        onRequestClose={onHideFulfill}
      >
        <Header
          modal
          left={<NavCloseButton />}
          onLeft={onHideFulfill}
          {...headerRightProps}
        />
        {this.renderContent()}
      </OBLightModal>
    );
  }
}

const mapStateToProps = state => ({
});

const mapDispatchToProps = {
  fulfillOrder,
};

export default connect(mapStateToProps, mapDispatchToProps)(FulfillModal);
