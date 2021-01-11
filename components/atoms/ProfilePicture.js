import React from 'react';
import { View } from 'react-native';
import { connect } from 'react-redux';
import * as _ from 'lodash';

import { badgeStyle } from '../../utils/navbar';
import NotificationBadge from './NotificationBadge';
import AvatarImage from '../atoms/AvatarImage';
import { brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  imageHolder: {
    width: 29,
    height: 29,
    flex: 1,
    flexDirection: 'column',
    justifyContent: 'flex-start',
    marginTop: 10,
  },
  image: {
    width: 29,
    height: 29,
  },
};

class ProfilePicture extends React.PureComponent {
  render() {
    const {
      profilePicture, unreadCount, focused, notifications,
    } = this.props;
    const counts = notifications.filter(notif => !notif.is_seen).length;
    return (
      <View>
        <View style={styles.imageHolder}>
          <AvatarImage
            style={{
              ...styles.image,
              shadowColor: focused ? brandColor : 'rgba(0, 0, 0, 0.5)',
              borderColor: focused ? brandColor : '#FFF'
            }}
            thumbnail={profilePicture}
            showLocal
          />
        </View>
        <NotificationBadge style={badgeStyle} notifCount={unreadCount + counts} />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profilePicture: _.get(state, 'profile.data.avatarHashes.tiny'),
  unreadCount: state.notifications.unread,
  notifications: state.stream.notifications,
});

export default connect(mapStateToProps)(ProfilePicture);
