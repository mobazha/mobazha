import React from 'react';
import { Alert, View, Text } from 'react-native';
import * as _ from 'lodash';
import { connect } from 'react-redux';

import { linkTextColor } from '../commonColors';

import { OBLightModal } from '../templates/OBModal';
import NavCloseButton from '../atoms/NavCloseButton';
import TextInput from '../atoms/TextInput';
import LinkText from '../atoms/LinkText';
import Header from '../molecules/Header';

import { openDispute } from '../../reducers/order';

import {I18n} from '../../langs/I18n';

const styles = {
  contentContainer: {
    paddingHorizontal: 17,
    flex: 1,
  },
  input: {
    fontSize: 17,
    height: '100%',
  },
};

class DisputeModal extends React.PureComponent {
  state = {
    note: '',
  }

  confirmDispute = () => {
    Alert.alert(
      'Submit dispute?',
      'The moderator will step in to help resolve the dispute. You can\'t undo this action',
      [
        { text: 'Cancel' },
        {
          text: 'Ok',
          onPress: () => {
            this.props.onHideDispute();
            setTimeout(this.doDispute, 500);
          },
        },
      ],
    );
  }

  doDispute = () => {
    const { note } = this.state;
    if (_.isEmpty(note)) {
      Alert.alert('', 'Please enter dispute reason!');
      return;
    }

    const { orderId, openDispute, orderType, vendorId } = this.props;
    openDispute({ orderId, claim: note, orderType, vendorId });
  }

  updateState = field => (value) => {
    const dict = {};
    dict[field] = value;
    this.setState(dict);
  }

  renderContent = () => {
    const { note } = this.state;
    return (
      <View style={styles.contentContainer}>
        <TextInput
          style={styles.input}
          noTitle
          placeholder="Why are you starting a dispute? Provide as much detail as possible."
          value={note}
          onChangeText={this.updateState('note')}
          noBorder
          multiline
          autoFocus
        />
      </View>
    );
  }

  render() {
    const { isOpen, onHideDispute } = this.props;
    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={isOpen}
        onRequestClose={onHideDispute}
      >
        <Header
          modal
          left={<NavCloseButton />}
          onLeft={onHideDispute}
          right={<LinkText text="Submit" color={linkTextColor} />}
          onRight={this.confirmDispute}
        />
        {this.renderContent()}
      </OBLightModal>
    );
  }
}

const mapDispatchToProps = {
  openDispute,
};

export default connect(null, mapDispatchToProps)(DisputeModal);
