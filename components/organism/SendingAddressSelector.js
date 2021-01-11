import React, { PureComponent } from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import { formLabelColor, linkTextColor, primaryTextColor, borderColor, brandColor } from '../commonColors';
import QRScanner from './QRScanner';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    borderTopLeftRadius: 8,
    borderTopRightRadius: 8,
    backgroundColor: 'white',
    padding: 16,
    paddingBottom: ifIphoneX(34, 0),
  },
  content: {
    flexDirection: 'row',
    alignItems: 'center',
    borderColor,
    borderWidth: 1,
    marginVertical: 16,
  },
  title: {
    marginTop: 8,
    fontSize: 15,
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  placeholder: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'italic',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  scanText: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: linkTextColor,
  },
  preview: {
    flex: 1,
    justifyContent: 'flex-end',
    alignItems: 'center',
  },
  input: {
    padding: 12,
    flex: 1,
    color: primaryTextColor,
  },
  rightIcon: {
    marginRight: 16,
  },
  sendButton: {
    backgroundColor: brandColor,
    paddingVertical: 16,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: 4,
  },
  sendButtonText: {
    fontSize: 16,
    fontWeight: 'bold',
    color: 'white',
    marginRight: 5,
    lineHeight: 18,
  },
  sendDisabled: {
    opacity: 0.5,
  },
  sendIcon: {
    height: 18,
    lineHeight: 18,
  },
};

export default class SendingAddressSelector extends PureComponent {
  static getDerivedStateFromProps(props) {
    return ({ address: props.address });
  }

  state = {
    address: '',
  };

  onChangeText = (address) => {
    this.setState({ address }, () => {
      this.props.onChange(address);
    });
  }

  setRef = (ref) => {
    this.txtInput = ref;
  }

  handleCodeDetected = (data) => {
    this.setState({ address: data.data }, () => this.props.onChange(data.data));
  }

  clearAddress() {
    this.txtInput.clear();
  }

  render() {
    const {
      title, placeholder, onSend,
    } = this.props;
    const { address } = this.state;
    return (
      <View style={styles.wrapper}>
        <Text style={styles.title}>{title}</Text>
        <View style={styles.content}>
          <TextInput
            style={styles.input}
            ref={this.setRef}
            placeholder={placeholder}
            value={address}
            onChangeText={this.onChangeText}
            underlineColorAndroid="transparent"
            autoFocus
          />
          <QRScanner style={styles.rightIcon} onQRCodeDetected={this.handleCodeDetected} fromWallet black />
        </View>
        <TouchableOpacity activeOpacity={1} onPress={onSend} disabled={address === ''}>
          <View style={[styles.sendButton, address === '' ? styles.sendDisabled : {}]}>
            <Text style={styles.sendButtonText}>{I18n.t('components.organism.SendingAddressSelector.send')}</Text>
            <Ionicons size={16} name="md-send" color="white" style={styles.sendIcon} />
          </View>
        </TouchableOpacity>
      </View>
    );
  }
}
