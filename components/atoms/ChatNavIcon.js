import React from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';

import NotificationBadge from './NotificationBadge';
import { tabBadgeStyle } from '../../utils/navbar';
import { chatsMap } from '../../selectors/chat';

import {I18n} from '../../langs/I18n';

const styles = {
  iconWrapper: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    width: 35,
  },
};

class ChatNavIcon extends React.PureComponent {
  render() {
    const { focused, chats } = this.props;
    const unreads = chats.data.reduce((result, item) => (result + (item.unread > 0 ? 1 : 0)), 0);

    return (
      <View style={styles.iconWrapper}>
        <Feather name="message-circle" size={23} color={focused ? '#00bf65' : '#969696'} />
        <NotificationBadge
          style={tabBadgeStyle}
          notifCount={unreads}
        />
      </View>
    );
  }
}

const mapStateToProps = chatsMap;

export default connect(mapStateToProps)(ChatNavIcon);
