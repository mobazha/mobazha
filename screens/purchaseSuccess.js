import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Image, Text, TouchableWithoutFeedback, TextInput, Alert } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { hasIn } from 'lodash';

import Header from '../components/molecules/Header';
import InputGroup from '../components/atoms/InputGroup';
import LinkText from '../components/atoms/LinkText';
import { primaryTextColor, linkTextColor, foregroundColor, brandColor } from '../components/commonColors';
import { screenWrapper } from '../utils/styles';

import { getGeneralCoinInfo } from '../utils/currency';
import { sendChat } from '../api/chat';
import { convertorsMap } from '../selectors/currency';
import NavBackButton from '../components/atoms/NavBackButton';
import { sendNotification } from '../reducers/stream';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'column',
    justifyContent: 'center',
    alignItems: 'center',
    marginVertical: 25,
  },
  sentAmountWrapper: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'flex-end',
    marginBottom: 9,
  },
  sentAmount: {
    fontSize: 17,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
  currencyCode: {
    fontSize: 12,
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
  iconWrapper: {
    width: 104,
    height: 104,
    borderRadius: 52,
    backgroundColor: 'white',
    shadowColor: 'rgba(213, 213, 213, 0.5)',
    shadowOffset: {
      width: 0,
      height: 2,
    },
    shadowRadius: 2,
    shadowOpacity: 1,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: 'rgb(211, 211, 211)',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 11,
  },
  descriptionWrapper: {
    flexDirection: 'row',
    justifyContent: 'center',
  },
  coinImage: {
    width: 20,
    height: 20,
    marginRight: 6,
  },
  description: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  toTransaction: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: linkTextColor,
  },
  messageWrapper: {
    paddingVertical: 12,
  },
  messageInput: {
    height: 48,
  },
  sendButton: {
    width: 60,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    borderRadius: 2,
    alignItems: 'center',
    padding: 12,
    justifyContent: 'center',
  },
  sendButtonText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
};

class PurchaseSuccess extends PureComponent {
  state = {
    message: '',
  };

  sendMessage() {
    const { message } = this.state;
    const { username, password } = this.props;
    const peerID = this.props.navigation.getParam('peerID');
    const orderInfo = this.props.navigation.getParam('orderInfo');
    sendChat(username, password, peerID, orderInfo.orderId, message).then((response) => {
      if (hasIn(response, 'messageId')) {
        Alert.alert(I18n.t('screens.purchaseSuccess.successfully_sent'));
      }
    });
    sendNotification({ verb: 'chat', type: 'sent', peerID, content: { body: I18n.t('screens.purchaseSuccess.received_message')} });
  }

  render() {
    const { localCurrency, localLabelFromBCH } = this.props;
    const price = this.props.navigation.getParam('price');
    const handle = this.props.navigation.getParam('handle');
    const coinInfo = getGeneralCoinInfo('BCH');
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title={I18n.t('screens.purchaseSuccess.sent')}
          right={<LinkText text={I18n.t('screens.purchaseSuccess.close')} />}
          onRight={() => {
            this.props.navigation.navigate('Store');
          }}
        />
        <InputGroup title={I18n.t('screens.purchaseSuccess.order_complete')}>
          <View style={styles.wrapper}>
            <View style={styles.sentAmountWrapper}>
              <Text style={styles.sentAmount}>{localLabelFromBCH(price, coinInfo.value)}</Text>
              <Text style={styles.currencyCode}>{` ${localCurrency}`}</Text>
            </View>
            <View style={styles.iconWrapper}>
              <Ionicons name="md-checkmark" color="rgb(0, 148, 2)" size={64} />
            </View>
            <View style={styles.descriptionWrapper}>
              <Image style={styles.coinImage} source={coinInfo.icon} />
              <Text style={styles.description}>{`${coinInfo.label} Sent`}</Text>
            </View>
            <Text style={styles.toTransaction}>{I18n.t('screens.purchaseSuccess.view_transaction')}</Text>
          </View>
        </InputGroup>
        <InputGroup title={`{I18n.t('screens.purchaseSuccess.message_for')} ${handle}`}>
          <View style={styles.messageWrapper}>
            <TextInput
              placeholder={I18n.t('screens.purchaseSuccess.provide_details')}
              placeholderTextColor="rgb(82, 82, 82)"
              multiline
              numberOfLines={4}
              style={styles.messageInput}
              onChangeText={(text) => {
                this.setState({
                  message: text,
                });
              }}
            />
            <TouchableWithoutFeedback
              onPress={() => {
                this.sendMessage();
              }}
            >
              <View style={styles.sendButton}>
                <Text style={styles.sendButtonText}>{I18n.t('screens.purchaseSuccess.send')}</Text>
              </View>
            </TouchableWithoutFeedback>
          </View>
        </InputGroup>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  ...convertorsMap(state),
});

export default connect(
  mapStateToProps,
  { sendNotification },
)(PurchaseSuccess);
