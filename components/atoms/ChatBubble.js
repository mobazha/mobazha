/* eslint no-use-before-define: ["error", { "variables": false }] */

import PropTypes from 'prop-types';
import React from 'react';
import { Text, Clipboard, TouchableWithoutFeedback, View } from 'react-native';
import { MessageImage, MessageText, Time } from 'react-native-gifted-chat';
import Ionicons from 'react-native-vector-icons/Ionicons';
import decode from 'unescape';

import { isSameDay, isSameUser } from '../../utils/chat';
import { formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const Color = {
  backgroundTransparent: 'transparent',
  leftBubbleBackground: '#f0f0f0',
  white: '#fff',
};

const styles = {
  checkmark: {
    marginRight: 5,
    marginBottom: 5,
    marginTop: 1.3,
  },
  alert: {
    marginLeft: 10,
    marginBottom: 16,
  },
  errorMessage: {
    fontSize: 12,
    color: 'rgb(255, 59, 48)',
    textAlign: 'right',
    marginTop: 6,
  },
  left: {
    container: {
      flex: 1,
      alignItems: 'flex-start',
      marginBottom: 20,
    },
    wrapper: {
      borderRadius: 5,
      backgroundColor: Color.leftBubbleBackground,
      marginRight: 60,
      minHeight: 20,
      justifyContent: 'flex-end',
    },
    containerToNext: {
      borderBottomLeftRadius: 3,
    },
    containerToPrevious: {
      borderTopLeftRadius: 3,
    },
  },
  right: {
    container: {
      flex: 1,
      alignItems: 'center',
      justifyContent: 'flex-end',
      flexDirection: 'row',
      marginBottom: 20,
    },
    wrapper: {
      borderRadius: 5,
      backgroundColor: '#f2fbf3',
      marginLeft: 60,
      minHeight: 20,
      justifyContent: 'flex-end',
    },
    containerToNext: {
      borderBottomRightRadius: 3,
    },
    containerToPrevious: {
      borderTopRightRadius: 3,
    },
  },
  bottom: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
  },
  tick: {
    fontSize: 10,
    backgroundColor: Color.backgroundTransparent,
    color: Color.white,
  },
  tickView: {
    flexDirection: 'row',
    marginRight: 10,
  },
  timeTextStyle: {
    right: {
      color: formLabelColor,
    },
  },
  orderInfoWrapper: {
    flexDirection: 'row',
    maxWidth: 250,
    minWidth: 150,
    alignItems: 'flex-end',
    marginHorizontal: 10,
    marginTop: 10,
  },
  username: {
    maxWidth: 150,
    fontWeight: 'bold',
    fontSize: 14,
    color: formLabelColor,
  },
  userType: {
    marginLeft: 10,
    fontWeight: 'normal',
    fontSize: 12,
    paddingBottom: 1,
    color: formLabelColor,
  },
};

export default class Bubble extends React.PureComponent {
  onLongPress = () => {
    const { onLongPress, currentMessage } = this.props;
    if (onLongPress) {
      onLongPress(this.context, currentMessage);
    } else if (currentMessage.text) {
      const options = ['Copy Text', 'Cancel'];
      const cancelButtonIndex = options.length - 1;
      this.context.actionSheet().showActionSheetWithOptions(
        {
          options,
          cancelButtonIndex,
        },
        (buttonIndex) => {
          switch (buttonIndex) {
            case 0:
              Clipboard.setString(currentMessage.text);
              break;
            default:
              break;
          }
        },
      );
    }
  }

  getBubbleNextStyle() {
    const {
      currentMessage, nextMessage, position, containerToNextStyle,
    } = this.props;
    if (
      isSameUser(currentMessage, nextMessage)
      && isSameDay(currentMessage, nextMessage)
    ) {
      return [
        styles[position].containerToNext,
        containerToNextStyle[position],
      ];
    }
    return null;
  }

  getBubblePreviousStyle() {
    const {
      currentMessage, previousMessage, position, containerToPreviousStyle,
    } = this.props;
    if (
      isSameUser(currentMessage, previousMessage)
      && isSameDay(currentMessage, previousMessage)
    ) {
      return [
        styles[position].containerToPrevious,
        containerToPreviousStyle[position],
      ];
    }
    return null;
  }

  renderMessageText() {
    const { currentMessage, renderMessageText } = this.props;
    if (currentMessage.text) {
      const { containerStyle, wrapperStyle, ...messageTextProps } = this.props;
      if (renderMessageText) {
        return renderMessageText(messageTextProps);
      }
      return <MessageText {...messageTextProps} />;
    }
    return null;
  }

  renderMessageImage() {
    const { currentMessage } = this.props;
    if (currentMessage.image) {
      const { containerStyle, wrapperStyle, ...messageImageProps } = this.props;
      return <MessageImage {...messageImageProps} />;
    }
    return null;
  }

  renderTime() {
    const { currentMessage } = this.props;
    if (currentMessage.createdAt) {
      const { containerStyle, wrapperStyle, ...timeProps } = this.props;
      return (
        <Time
          {...timeProps}
          textStyle={styles.timeTextStyle}
        />
      );
    }
    return null;
  }

  renderOrderInfo() {
    const {
      currentMessage: { user: { name, _id: userId } },
      position,
      orderProps: {
        isFromOrder, moderatorId, vendorId, buyerId,
      },
    } = this.props;
    if (!isFromOrder || position === 'right') {
      return false;
    }
    let userType = '';
    switch (userId) {
      case moderatorId:
        userType = 'moderator';
        break;
      case vendorId:
        userType = 'seller';
        break;
      case buyerId:
      default:
        userType = 'buyer';
    }
    return (
      <View style={styles.orderInfoWrapper}>
        <Text style={styles.username} numberOfLines={1} ellipsizeMode="tail">{decode(name)}</Text>
        <Text style={styles.userType}>{userType}</Text>
      </View>
    );
  }

  render() {
    const {
      loading, success, position,
      containerStyle, wrapperStyle,
      onPress, touchableProps, bottomContainerStyle,
    } = this.props;
    return (
      <View style={[styles[position].container, containerStyle[position]]}>
        <View>
          <View
            style={[
              styles[position].wrapper,
              wrapperStyle[position],
              this.getBubbleNextStyle(),
              this.getBubblePreviousStyle(),
            ]}
          >
            <TouchableWithoutFeedback
              onLongPress={this.onLongPress}
              onPress={onPress}
              accessibilityTraits="text"
              {...touchableProps}
            >
              <View>
                {this.renderOrderInfo()}
                {this.renderMessageImage()}
                {this.renderMessageText()}
                <View style={[styles.bottom, bottomContainerStyle[position]]}>
                  {this.renderTime()}
                  {position === 'right' && loading && (
                    <Ionicons style={styles.checkmark} name="md-time" size={10} color={formLabelColor} />
                  )}
                  {position === 'right' && !loading && success && (
                    <Ionicons style={styles.checkmark} name="md-checkmark" size={10} color="#8cd985" />
                  )}
                </View>
              </View>
            </TouchableWithoutFeedback>
          </View>
          {position === 'right' && !loading && !success && (
            <Text style={styles.errorMessage}>{I18n.t('components.atoms.ChatBubble.fail_retry')}</Text>
          )}
        </View>
        {position === 'right' && !loading && !success && (
          <Ionicons style={styles.alert} name="md-alert" size={14} color="rgb(255, 59, 48)" />
        )}
      </View>
    );
  }
}

Bubble.contextTypes = {
  actionSheet: PropTypes.func,
};

Bubble.defaultProps = {
  touchableProps: {},
  onLongPress: null,
  renderMessageText: null,
  position: 'left',
  currentMessage: {
    text: null,
    createdAt: null,
    image: null,
  },
  nextMessage: {},
  previousMessage: {},
  containerStyle: {},
  wrapperStyle: {},
  bottomContainerStyle: {},
  tickStyle: {},
  containerToNextStyle: {},
  containerToPreviousStyle: {},
};
