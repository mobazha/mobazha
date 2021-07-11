import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, Animated } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import AnimateNumber from 'react-native-animate-number';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import {
  primaryTextColor,
  foregroundColor,
  formLabelColor,
  brandColor,
  warningColor,
  borderColor,
} from '../commonColors';
import SupportHaven from '../organism/SupportHaven';
import OBSlidingPanel from '../atoms/OBSlidingPanel';
import PayPanel from '../organism/PayPanel';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    flexDirection: 'column',
    justifyContent: 'center',
    alignItems: 'center',
  },
  iconWrapper: {
    width: 110,
    height: 110,
    justifyContent: 'center',
    alignItems: 'center',
  },
  title: {
    fontSize: 16,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  description: {
    marginTop: 16,
    fontSize: 15,
    fontWeight: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
    width: 280,
    textAlign: 'center',
  },
  orderDetailsButton: {
    marginTop: 16,
    paddingHorizontal: 20,
    paddingVertical: 8,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor,
    justifyContent: 'center',
    borderRadius: 4,
  },
  buttonText: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  retryButton: {
    marginTop: 12,
    width: 250,
    paddingVertical: 9,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
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
  loadingWheel: {
    width: 70,
    height: 70,
    borderRadius: 35,
    borderTopWidth: 6,
    borderLeftWidth: 6,
    borderBottomWidth: 6,
    borderColor: primaryTextColor,
    marginBottom: 12,
  },
  errorDescription: {
    fontSize: 13,
    color: formLabelColor,
    width: 280,
    marginBottom: 40,
  },
  errorTitle: {
    fontWeight: 'bold',
  },
  loadingWrapper: {
    position: 'relative',
    width: 70,
    height: 70,
  },
  numberWrapper: {
    position: 'absolute',
    width: 70,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
    top: 15,
    left: 0,
  },
};

export default class PurchaseState extends React.PureComponent {
  state = {
    loading: true,
  };

  componentDidMount() {
    this.animate();
  }

  onFinish = () => {
    const { onFinish, percentage } = this.props;
    if (percentage === 100) {
      this.setState({ loading: false });
      onFinish();
    }
  }

  aniVal = new Animated.Value(0);

  handleRetry = () => {
    this.props.onRetry();
    this.animate();
  }

  animate = () => {
    const { loading } = this.state;
    Animated.timing(this.aniVal, {
      duration: 1000,
      toValue: 1,
    }).start(() => {
      this.aniVal.setValue(0);
      if (loading) {
        this.animate();
      }
    });
  }

  handleGoToOrder = () => {
    const { navigation, paidOrderInfo: { orderId } } = this.props;
    navigation.navigate({
      routeName: 'OrderDetails',
      params: {
        orderId,
        orderType: 'purchases',
      },
    });
  }

  percentageFormatter = val => `${val}%`;

  renderLoading = () => {
    const { paymentState, percentage } = this.props;
    return (
      <View style={styles.loadingWrapper}>
        <Animated.View
          style={[
              styles.loadingWheel,
              {
                transform: [
                  {
                    rotate: this.aniVal.interpolate({ inputRange: [0, 1], outputRange: ['0deg', '360deg'] }),
                  },
                ],
              },
            ]}
        />
        {paymentState !== 'error' && (
          <View style={styles.numberWrapper}>
            <AnimateNumber
              value={percentage}
              countBy={1}
              interval={50}
              onFinish={this.onFinish}
              formatter={this.percentageFormatter}
            />
          </View>
        )}
      </View>
    );
  }

  render() {
    const { paymentState, reason, onAmountSelected } = this.props;
    const { loading } = this.state;
    let icon;
    let title;
    let description;
    if (paymentState === 'success') {
      icon = <Ionicons name="md-checkmark" color={brandColor} size={110} />;
      title = I18n.t('components.templates.PurchaseState.thank_you');
      description = I18n.t('components.templates.PurchaseState.order_placed');
    } else if (paymentState === 'pending') {
      icon = this.renderLoading();
      title = I18n.t('components.templates.PurchaseState.processing');
      description = I18n.t('components.templates.PurchaseState.hang_tight');
    } else { // in this case paymentState is 'error' always
      icon = <Ionicons name="md-close" color={warningColor} size={110} />;
      title = 'Uh oh!';
      description =I18n.t('components.templates.PurchaseState.transaction_failed');
    }

    return (
      <View style={styles.wrapper}>
        <View style={{ flex: 0.2 }} />
        <View style={styles.iconWrapper}>{icon}</View>
        <Text style={styles.title}>{title}</Text>
        <Text style={styles.description}>{description}</Text>
        {paymentState === 'error' && (
          <TouchableWithoutFeedback onPress={this.handleRetry}>
            <View style={styles.retryButton} >
              <Text style={styles.buttonText}>{I18n.t('components.templates.PurchaseState.retry')}</Text>
            </View>
          </TouchableWithoutFeedback>
        )}
        {paymentState === 'success' && (
          <TouchableWithoutFeedback onPress={this.handleGoToOrder}>
            <View style={styles.orderDetailsButton}>
              <Text style={styles.buttonText}>{I18n.t('components.templates.PurchaseState.order_details')}</Text>
            </View>
          </TouchableWithoutFeedback>
        )}
        <View style={{ flex: 1 }} />
        {paymentState === 'success' && (
          <SupportHaven onAmountSelected={onAmountSelected} />
        )}
        {paymentState === 'error' && (
          <Text style={styles.errorDescription}>
            <Text style={styles.errorTitle}>{I18n.t('components.templates.PurchaseState.error')}</Text>
            {` ${reason}`}
          </Text>
        )}
      </View>
    );
  }
}
