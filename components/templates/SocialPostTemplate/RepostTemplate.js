import React from 'react';
import { Alert } from 'react-native';
import { connect } from 'react-redux';

import OBActionSheet from '../../organism/ActionSheet';
import FeedPreview from '../../molecules/FeedPreview';

import { repostFeed, deleteRepost } from '../../../reducers/feed';

import { eventTracker } from '../../../utils/EventTracker';
import { convertFromReference } from '../../../utils/feed';
import SocialPostBase from './SocialPostBase';

import {I18n} from '../../../langs/I18n';
class RepostTemplate extends React.Component {
  state = { comment: '', showModal: false, reference: '' };

  setActionSheet = (ref) => { this.actionSheet = ref; }

  handleChangeComment = (comment) => { this.setState({ comment }); }

  handleSetInputRef = (ref) => { this.inputBox = ref; }

  repost = (reference, activityId) => {
    this.setState({ reference, activityId });
    this.actionSheet.show();
  }

  handleChange = (index) => {
    const { repostFeed, profile, onAddRepost } = this.props;
    const { peerID: repostID } = profile;
    const { reference, activityId } = this.state;
    switch (index) {
      case 0: {
        repostFeed({ reference, activityId, repostID, comment: '', success: onAddRepost });
        break;
      }
      case 1: {
        this.setState({ showModal: true, comment: '' });
        setTimeout(() => { this.inputBox.focus(); }, 500);
        break;
      }
      default: break;
    }
  }

  handleRepostFeed = () => {
    const { repostFeed, profile, onAddRepost } = this.props;
    const { peerID: repostID } = profile;
    const { comment, reference, activityId } = this.state;
    eventTracker.trackEvent('Social-RepostedPost');
    repostFeed({ reference, repostID, activityId, comment, success: onAddRepost });
    this.setState({ showModal: false });
  }

  deleteRepost = (reference, activityId, reactionId, originalActivityId) => {
    this.setState({ reference, activityId, reactionId, originalActivityId });
    Alert.alert(I18n.t('components.templates.SocialPostTemplate.RePostTemplate.delete_feed'),
      I18n.t('components.templates.SocialPostTemplate.RePostTemplate.delete_repost'), [
      { text: I18n.t('components.templates.SocialPostTemplate.RePostTemplate.cancel')},
      {
        text: I18n.t('components.templates.SocialPostTemplate.RePostTemplate.delete'),
        onPress: this.doDeleteRepost,
      },
    ]);
  }

  doDeleteRepost = () => {
    const { deleteRepost, onDeleteRepost } = this.props;
    const { reference, activityId, reactionId, originalActivityId } = this.state;
    deleteRepost({
      reference,
      activityId,
      reactionId,
      originalActivityId,
      success: onDeleteRepost,
    });
  }

  handleHideModal = () => {
    this.setState({ showModal: false });
  }

  render() {
    const { showModal, activityId, reference, comment } = this.state;
    const { peerID } = convertFromReference(reference);
    return (
      <React.Fragment>
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={[I18n.t('components.templates.SocialPostTemplate.RePostTemplate.repost'),
          I18n.t('components.templates.SocialPostTemplate.RePostTemplate.repost_with_comment'),
          I18n.t('components.templates.SocialPostTemplate.RePostTemplate.cancel')]}
          cancelButtonIndex={2}
        />
        <SocialPostBase
          showModal={showModal}
          comment={comment}
          onHideModal={this.handleHideModal}
          onChangeComment={this.handleChangeComment}
          setInputRef={this.handleSetInputRef}
          onPost={this.handleRepostFeed}
        >
          <FeedPreview peerID={peerID} activityId={activityId} />
        </SocialPostBase>
      </React.Fragment>
    );
  }
}

const mapStateToProps = state => ({ profile: state.profile.data || {} });
const mapDispatchToProps = { repostFeed, deleteRepost };

export default connect(
  mapStateToProps,
  mapDispatchToProps,
  null,
  { withRef: true },
)(RepostTemplate);
