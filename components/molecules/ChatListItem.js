import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { timeSince } from '../../utils/time';
import AvatarImage from '../atoms/AvatarImage';

import { cellStyles, chatStyles } from '../../utils/styles';
import NotificationBadge from '../atoms/NotificationBadge';
import { formLabelColor } from '../commonColors';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  titleContainer: {
    flexDirection: 'row',
    marginBottom: 4,
  },
  timestampWrapper: {
    paddingVertical: 6,
    marginRight: 13,
    width: 32,
    height: '100%',
    alignItems: 'flex-end',
    justifyContent: 'space-between',
  },
  contentContainer: {
    paddingVertical: 10,
    alignItems: 'flex-start',
  },
};

export default ({
  item, profile, style, navigation,
}) => {
  const name = _.get(profile, 'name');
  const handle = _.get(profile, 'handle');
  const {
    unread, outgoing, loading, success,
  } = item;
  const isUnread = unread > 0;
  const onPress = () => {
    eventTracker.trackEvent('Chat-ViewChat');
    navigation.navigate('ChatDetail', { peerID: item.peerId, subject: item.subject || '' });
  };
  return (
    <TouchableWithoutFeedback
      onPress={onPress}
    >
      <View style={[chatStyles.wrapper, style]}>
        <View style={[cellStyles.contentContainer, styles.contentContainer]}>
          <View style={styles.avatarWrapper}>
            <AvatarImage
              style={chatStyles.avatarImage}
              thumbnail={_.get(profile, 'avatarHashes.tiny')}
              onPress={() => {
                navigation.navigate('ChatDetail', { peerID: item.peerId, subject: item.subject || '' });
              }}
            />
          </View>
          <View style={chatStyles.textWrapper}>
            <View style={styles.titleContainer}>
              <Text style={chatStyles.name} numberOfLines={1}>
                {decode(name)}
              </Text>
              {!!handle && (
                <Text style={chatStyles.handle} numberOfLines={1}>
                  {` @${handle}`}
                </Text>
              )}
              <View style={{ flex: 1 }} />
              {outgoing && (
                <React.Fragment>
                  {loading ? (
                    <Ionicons style={styles.checkmark} name="md-time" size={12} color={formLabelColor} />
                  ) : (loading === false && success === false) ? (
                    <Ionicons style={styles.alert} name="md-alert" size={12} color="rgb(255, 59, 48)" />
                  ) : (
                    <Ionicons style={styles.checkmark} name="md-checkmark" size={12} color="#8cd985" />
                  )}

                </React.Fragment>
              )}
            </View>
            <Text style={[chatStyles.handle, isUnread && chatStyles.unread]} numberOfLines={3}>
              {decode(item.lastMessage)}
            </Text>
          </View>
          <View style={styles.timestampWrapper}>
            <Text style={isUnread && chatStyles.unread}>
              {timeSince(new Date(item.timestamp), true)}
            </Text>
            <NotificationBadge notifCount={unread} noBorder />
          </View>
        </View>
        <View style={cellStyles.separatorContainer}>
          <View style={cellStyles.separator} />
        </View>
      </View>
    </TouchableWithoutFeedback>
  );
};
