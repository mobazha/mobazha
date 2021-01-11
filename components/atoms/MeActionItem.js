import React from 'react';
import { TouchableWithoutFeedback, View, Text, Dimensions } from 'react-native';
import { withNavigation } from 'react-navigation';
import { connect } from 'react-redux';

import { greenTintColor, brandColor } from '../commonColors';
import NotificationBadge from './NotificationBadge';
import { badgeStyle } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('screen');

const styles = {
  item: {
    width: SCREEN_WIDTH / 4,
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'flex-end',
    paddingVertical: 17.5,
  },
  itemContent: {
    alignItems: 'center',
  },
  icon: {
    textAlign: 'center',
    paddingLeft: 1,
  },
  text: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#767676',
    marginTop: 7,
  },
  iconWrapper: {
    alignItems: 'center',
    justifyContent: 'center',
    width: 50,
    height: 50,
    borderRadius: 25,
    backgroundColor: greenTintColor,
  },
};

class MeActionItem extends React.PureComponent {
  render() {
    const { item, onPress, unreadCount, notifications } = this.props;
    const { icon: Icon, caption, isNotif } = item;
    const counts = notifications.filter(notif => !notif.is_seen).length;
    return (
      <TouchableWithoutFeedback onPress={onPress} delayPressIn={200}>
        <View style={styles.item}>
          {isNotif ? (
            <View>
              <View style={styles.iconWrapper}>
                <Icon color={brandColor} size={24} style={styles.icon} />
              </View>
              <NotificationBadge style={badgeStyle} notifCount={unreadCount + counts} />
            </View>
          ) : (
            <View style={styles.iconWrapper}>
              <Icon color={brandColor} size={24} style={styles.icon} />
            </View>
          )}
          <Text style={styles.text}>{caption}</Text>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  unreadCount: state.notifications.unread,
  notifications: state.stream.notifications,
});

export default withNavigation(connect(mapStateToProps)(MeActionItem));
