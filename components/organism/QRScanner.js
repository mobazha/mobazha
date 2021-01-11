import React, { PureComponent } from 'react';
import { Text, View, TouchableWithoutFeedback, Image } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { RNCamera } from 'react-native-camera';
import { withNavigation } from 'react-navigation';

import { OBDarkModal } from '../templates/OBModal';
import ModalBackButton from '../atoms/ModalBackButton';

import { navHeightStyle } from '../../utils/navbar';
import { handleOBDeeplinkWithNavigation } from '../../utils/navigation';

import qrcodeIcon from '../../assets/icons/qrcode.png';

import {I18n} from '../../langs/I18n';

const styles = {
  qrcodeIcon: {
    width: 20,
    height: 20,
  },
  preview: {
    marginTop: navHeightStyle.height,
    flex: 1,
    justifyContent: 'flex-end',
    alignItems: 'center',
  },
  overlay: {
    position: 'absolute',
    top: navHeightStyle.height,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: 'center',
    alignItems: 'center',
  },
  tooltip: {
    color: '#fff',
    textAlign: 'center',
  },
};

class QRScanner extends PureComponent {
  state = {
    showScanner: false,
  };

  handleClose = () => {
    this.setState({ showScanner: false });
  };

  handleOpenScanner = () => {
    this.setState({ showScanner: true });
  };

  handleCodeDetected = (data) => {
    const { onQRCodeDetected } = this.props;
    if (onQRCodeDetected) {
      onQRCodeDetected(data);
    } else {
      const url = data.data;
      handleOBDeeplinkWithNavigation(url, this.props.navigation);
    }

    this.handleClose();
  }

  render() {
    const { black, style, fromWallet } = this.props;
    const { showScanner } = this.state;

    return (
      <View style={style}>
        <TouchableWithoutFeedback onPress={this.handleOpenScanner}>
          <Image style={[styles.qrcodeIcon, black ? { tintColor: 'black' } : {}]} source={qrcodeIcon} />
        </TouchableWithoutFeedback>
        <OBDarkModal
          visible={showScanner}
          onDismiss={this.handleClose}
          onRequestClose={this.handleClose}
          darkContent
        >
          <RNCamera
            type={RNCamera.Constants.Type.back}
            barCodeTypes={[RNCamera.Constants.BarCodeType.qr]}
            onBarCodeRead={this.handleCodeDetected}
            style={styles.preview}
          />
          <View style={styles.overlay}>
            <Ionicons name="ios-qr-scanner" size={300} color="#FFF" />
            <Text style={styles.tooltip}>
              {fromWallet ? (
                I18n.t('components.organism.QRScanner.scan_qr_payment_address')
              ) : (
                I18n.t('components.organism.QRScanner.scan_qr_store')
              )}
            </Text>
          </View>
          <ModalBackButton onPress={this.handleClose} />
        </OBDarkModal>
      </View>
    );
  }
}

export default withNavigation(QRScanner);
