import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, Share, Platform } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';
import * as _ from 'lodash';

import { secondaryTextColor, foregroundColor, brandColor } from '../commonColors';
import { likePost, unlikePost } from '../../reducers/stream';
import { signMessage } from '../../api/feed';
import { eventTracker } from '../../utils/EventTracker';
import { createFeedUrlFromPeerIDAndSlug } from '../../utils/navigation';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    width: '100%',
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: foregroundColor,
    justifyContent: 'space-between',
    paddingRight: 15,
  },
  btn: {
    paddingVertical: 10,
    flexDirection: 'row',
    width: 50,
  },
  buttonIcon: {
    marginRight: 5,
  },
  btnText: {
    fontSize: 14,
    fontStyle: 'normal',
    textAlign: 'left',
    color: secondaryTextColor,
    paddingTop: (Platform.OS === 'ios' ? 2 : 0),
  },
  activeText: {
    fontWeight: 'bold',
    color: brandColor,
  },
  likes: {
    marginLeft: 'auto',
  },
};

class FeedItemFooter extends React.PureComponent {
  getReference = () => {
    const { item } = this.props;
    const postData = _.get(item, 'object.data.post') || _.get(item, 'data.post');
    const { slug, vendorID: { peerID } } = postData;
    return `${peerID}/${slug}`;
  }

  getReactionStatus = () => {
    const { item } = this.props;
    return _.get(item, 'reaction_counts', {});
  }

  share = () => {
    const { item } = this.props;
    const { actor: { id: peerID }, id } = item;
    const { status } = _.get(item, 'object.data', {});
    Share.share({
      message: createFeedUrlFromPeerIDAndSlug(peerID, id),
      title: status,
    });
  }

  repost = () => {
    const { repost, deleteRepost, item: { id, own_reactions } } = this.props;
    if (this.isReposted()) {
      const reactionId = _.get(own_reactions, 'repost[0].id', false);
      const activityId = _.get(own_reactions, 'repost[0].data.activityId');
      if (reactionId) {
        const reference = _.get(own_reactions, 'repost[0].data.reference');
        deleteRepost(reference, activityId, reactionId, id);
      }
    } else {
      const reference = this.getReference();
      repost(reference, id);
    }
  }

  isLiked = () => {
    const { item } = this.props;
    const ownLikes = _.get(item, 'own_reactions.like', []);
    return ownLikes.length > 0;
  }

  isReposted = () => {
    const { item } = this.props;
    const ownReposts = _.get(item, 'own_reactions.repost', []);
    return ownReposts.length > 0;
  }

  isCommented = () => {
    const { item } = this.props;
    const ownComments = _.get(item, 'own_reactions.comment', []);
    return ownComments.length > 0;
  }

  comment = () => {
    const { toPost, activityId } = this.props;
    toPost(activityId, 'comment', true);
  }

  like = async () => {
    const { likePost, activityId, item = {}, unlikePost } = this.props;

    if (this.isLiked()) {
      const { actor } = item;
      const reactionId = _.get(item, 'own_reactions.like[0].id');
      unlikePost({ reactionId, activityId, targetPeerID: actor });
      eventTracker.trackEvent('Social-UnlikedPost');
    } else {
      const { foreignId, actor } = item;
      const signature = await signMessage(foreignId);
      likePost({ signature, activityId, targetPeerID: actor });
      eventTracker.trackEvent('Social-LikedPost');
    }
  }

  render() {
    const { style } = this.props;
    const { like = 0, repost = 0, comment = 0 } = this.getReactionStatus();
    const isCommented = this.isCommented();
    const isReposted = this.isReposted();
    const isLiked = this.isLiked();
    return (
      <View style={[styles.wrapper, style]}>
        <TouchableWithoutFeedback onPress={this.comment}>
          <View style={styles.btn}>
            <Feather
              style={styles.buttonIcon}
              name={Platform.OS === 'ios' ? 'message-circle' : 'message-square'}
              size={19}
              color={isCommented ? brandColor : secondaryTextColor}
            />
            {comment > 0 && <Text style={[styles.btnText, isCommented && styles.activeText]}>{comment}</Text>}
          </View>
        </TouchableWithoutFeedback>
        <TouchableWithoutFeedback onPress={this.repost}>
          <View style={styles.btn}>
            <Feather
              style={styles.buttonIcon}
              name="repeat"
              size={19}
              color={isReposted ? brandColor : secondaryTextColor}
            />
            {repost > 0 && <Text style={[styles.btnText, isReposted && styles.activeText]}>{repost}</Text>}
          </View>
        </TouchableWithoutFeedback>
        <TouchableWithoutFeedback onPress={this.like}>
          <View style={styles.btn}>
            <Feather
              style={styles.buttonIcon}
              name="thumbs-up"
              size={19}
              color={isLiked ? brandColor : secondaryTextColor}
            />
            {like > 0 && <Text style={[styles.btnText, isLiked && styles.activeText]}>{like}</Text>}
          </View>
        </TouchableWithoutFeedback>
        <TouchableWithoutFeedback onPress={this.share}>
          <View style={styles.btn}>
            <Feather
              style={styles.buttonIcon}
              name={Platform.OS === 'ios' ? 'share' : 'share-2'}
              size={19}
              color={secondaryTextColor}
            />
          </View>
        </TouchableWithoutFeedback>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data || {},
});

const mapDispatchToProps = { likePost, unlikePost };

export default connect(mapStateToProps, mapDispatchToProps)(FeedItemFooter);
