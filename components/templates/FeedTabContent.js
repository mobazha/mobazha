import React from 'react';
import { connect } from 'react-redux';
import { Platform, View, Text, FlatList, ActivityIndicator } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';

import Comment from '../atoms/Comment';
import { foregroundColor, secondaryTextColor } from '../commonColors';
import FeedListItem from '../molecules/FeedListItem';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: foregroundColor,
    marginBottom: 60,
  },
  contentWrapper: {
    padding: 13,
  },
  list: {
    flex: 1,
  },
  placeholder: {
    alignItems: 'center',
    paddingTop: 45,
    flex: 1,
  },
  placeholderText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
    marginTop: 10,
  },
};

class FeedTabContent extends React.Component {
  state = {
    comment: undefined,
    posting: false,
  }

  posting(comment) {
    const { profile } = this.props;
    this.setState({
      comment: {
        loading: true,
        user_id: profile.peerID,
        data: {
          text: comment,
          createdAt: new Date(),
        },
      },
      posting: true,
    });
  }

  postSuccess() {
    this.setState({ comment: null, posting: false });
  }

  keyExtractor = (item, index) => `comment_${index}`;

  renderItem = ({ item }) => {
    const { type, onDeleteComment } = this.props;
    return type === 'comments' ? (
      <Comment item={item} onDeleteComment={onDeleteComment} />
    ) : (
      <FeedListItem item={item} />
    );
  }

  renderEmptyState = () => {
    const { fetching, type } = this.props;
    let text = '';
    switch (type) {
      case 'comments':
        text = I18n.t('components.templates.FeedTabContent.first_comment');
        break;
      case 'likes':
        text = I18n.t('components.templates.FeedTabContent.first_likes');
        break;
      default:
        text = I18n.t('components.templates.FeedTabContent.first_repost');
        break;
    }
    return fetching ? (
      <View style={styles.placeholder}>
        <ActivityIndicator size="large" />
      </View>
    ) : (
      <View style={styles.wrapper}>
        <View style={styles.placeholder}>
          <Feather
            name={Platform.OS === 'ios' ? 'message-circle' : 'message-square'}
            size={50}
            color={secondaryTextColor}
          />
          <Text style={styles.placeholderText}>{text}</Text>
        </View>
      </View>
    );
  }

  render() {
    const { data, type } = this.props;
    const { posting, comment } = this.state;
    const showingData = posting && type === 'comments' ? [...data, comment] : data;
    if (!showingData || showingData.length === 0) {
      return this.renderEmptyState();
    }
    return (
      <FlatList
        style={styles.contentWrapper}
        data={showingData}
        keyExtractor={this.keyExtractor}
        renderItem={this.renderItem}
        extraData={showingData}
      />
    );
  }
}

const mapStateToProps = state => ({ profile: state.profile.data });

export default connect(mapStateToProps, null, null, { withRef: true })(FeedTabContent);
