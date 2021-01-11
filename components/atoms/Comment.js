import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import { withNavigation } from 'react-navigation';
import { connect } from 'react-redux';
import { get } from 'lodash';
import moment from 'moment';
import ParsedText from 'react-native-parsed-text';
import decode from 'unescape';

import { loadingBackgroundColor, primaryTextColor, secondaryTextColor } from '../commonColors';
import AvatarImage from './AvatarImage';
import OBActionSheet from '../organism/ActionSheet';

import { deleteComment } from '../../reducers/stream';
import { showToast, hideToast } from '../../reducers/appstate';

import { formatSeconds } from '../../utils/time';
import { getOBParsePatterns } from '../../utils/parseText';
import { handleOBDeeplinkWithNavigation } from '../../utils/navigation';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    backgroundColor: 'white',
    marginBottom: 11,
  },
  loadingBg: {
    backgroundColor: loadingBackgroundColor,
  },
  content: {
    flex: 1,
    flexDirection: 'column',
    alignItems: 'flex-start',
    marginTop: 4,
  },
  topPart: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  name: {
    flex: 1,
    fontSize: 14,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  time: {
    fontSize: 14,
    color: secondaryTextColor,
  },
  comment: {
    fontSize: 14,
    color: primaryTextColor,
  },
  loading: {
    fontSize: 14,
    fontStyle: 'italic',
    color: secondaryTextColor,
  },
  avatarImage: {
    width: 38,
    height: 38,
    margin: 4,
    marginRight: 13,
  },
  linkText: {
    textDecorationLine: 'underline',
  },
};

class Comment extends React.Component {
  onPressAvatar = () => {
    const { navigation, profile, item } = this.props;
    const { user_id: peerID } = item;
    const currentPeerID = get(profile, 'peerID');
    if (peerID !== currentPeerID) {
      navigation.push('ExternalStore', { peerID });
    }
  }

  getProfile = () => {
    const { item } = this.props;
    return get(item, 'user.data', {});
  }

  setActionSheet = (ref) => {
    this.actionSheet = ref;
  }

  isMyComment = () => {
    const { userProfile } = this.props;
    const profile = this.getProfile();
    return userProfile.peerID === profile.peerID;
  }

  handleDeeplinkPress = (deeplink) => {
    const { navigation } = this.props;
    handleOBDeeplinkWithNavigation(deeplink, navigation);
  }

  handleCrypoLink = coin => (url) => {
    this.props.navigation.navigate('SendMoney', { coin, address: url });
  }

  handleRemove = () => {
    const { item = {}, deleteComment, onDeleteComment, showToast, hideToast } = this.props;
    const { activity_id: activityId, id: reactionId } = item;
    deleteComment({ activityId, reactionId, onSuccess: onDeleteComment });
    showToast({ type: 'message', message: 'Comment deleted' });
    setTimeout(hideToast, 3000);
  }

  handleChange = (index) => {
    if (index === 0) {
      this.handleRemove();
    }
  }

  showActionSheet = () => {
    if (this.actionSheet) {
      this.actionSheet.show();
    }
  }

  renderContent = () => {
    const { item } = this.props;
    const profile = this.getProfile();
    const name = get(profile, 'name', 'Anonymous');
    const { created_at: timestamp, loading } = item;
    const { text: comment } = item.data || {};
    const timeDiff = moment().diff(moment(timestamp), 'second');
    return (
      <View style={styles.content}>
        <View style={styles.topPart}>
          <Text style={styles.name}>{decode(name)}</Text>
          {loading ? (
            <Text style={styles.loading}>{I18n.t('components.atoms.Comment.posting')}</Text>
          ) : (
            <Text style={styles.time}>{formatSeconds(timeDiff)}</Text>
          )}
        </View>
        <ParsedText
          style={styles.comment}
          parse={getOBParsePatterns(styles.linkText, this.handleDeeplinkPress, this.handleCrypoLink)}
        >
          {decode(comment)}
        </ParsedText>
      </View>
    );
  }

  render() {
    const profile = this.getProfile();
    const thumbnail = get(profile, 'avatarHashes.tiny');
    const { loading } = this.props.item || {};
    return (
      <React.Fragment>
        <TouchableWithoutFeedback onPress={this.showActionSheet} disabled={!this.isMyComment()}>
          <View style={[styles.wrapper, loading && styles.loadingBg]}>
            <AvatarImage
              style={styles.avatarImage}
              thumbnail={thumbnail}
              onPress={this.onPressAvatar}
            />
            {this.renderContent()}
          </View>
        </TouchableWithoutFeedback>
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={['Delete', 'Cancel']}
          destructiveButtonIndex={0}
          cancelButtonIndex={1}
        />
      </React.Fragment>
    );
  }
}

const mapStateToProps = state => ({ userProfile: state.profile.data });
const mapDispatchToProps = { deleteComment, showToast, hideToast };

export default withNavigation(
  connect(mapStateToProps, mapDispatchToProps)(Comment),
);
