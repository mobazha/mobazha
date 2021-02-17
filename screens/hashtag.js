import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';
import { NavigationEvents } from 'react-navigation';

import GlobalFeedTemplate from '../components/templates/GlobalFeed';
import HashtagHeader from '../components/molecules/HashtagHeader';

import { fetchStream, updateSort } from '../reducers/stream';
import { setStreamBuildNotificationSeen } from '../reducers/appstate';
import { getFilteredList } from '../selectors/stream';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
class Hashtag extends PureComponent {
  constructor(props) {
    super(props);
    this.state = { sort: props.hashtagSort || 'recent', hashtag: '' };
  }

  componentDidMount() {
    const originHashtag = this.props.navigation.getParam('hashtag');
    const hashtag = originHashtag.trim();
    this.setState({ hashtag });
  }

  componentDidUpdate(prevProps, prevState) {
    const { hashtag: prevHashtag } = prevState;
    const { hashtag } = this.state;
    if (hashtag !== prevHashtag) {
      this.initStreamFeed();
    }
  }

  setGlobalFeedRef = (ref) => { if (ref) { this.globalFeed = ref.wrappedInstance; } }

  initStreamFeed = () => {
    const { sort, hashtag } = this.state;
    const { fetchStream } = this.props;
    const trimedText = hashtag.trim();
    if (trimedText.length > 0) {
      const withoutHash = trimedText[0] === '#' ? trimedText.slice(1) : trimedText;
      if (withoutHash !== '') {
        fetchStream({ filter: 'hashtag', hashtag: withoutHash, sort, appending: true });
      }
    }
  }

  handleChangeHash = (hashtag) => {
    this.setState({ hashtag });
  }

  handleFocus = () => {
    const originHashtag = this.props.navigation.getParam('hashtag');
    const hashtag = originHashtag.trim();
    this.setState({ hashtag });
  }

  handleBack = () => {
    this.props.navigation.goBack();
  }

  handleChangeSort = (sort) => {
    this.setState({ sort }, this.handleSortChanged);
  }

  handleSortChanged = () => {
    const { updateSort } = this.props;
    const { hashtag, sort } = this.state;
    if (this.globalFeed) {
      updateSort({ filter: 'hashtag', hashtag: hashtag.trim().slice(1), sort, appending: true });
    }
  }

  render() {
    const { getFilteredList } = this.props;
    const { sort, hashtag } = this.state;
    const feedList = getFilteredList('hashtag', sort);
    return (
      <View style={screenWrapper.wrapper}>
        <NavigationEvents onDidFocus={this.handleFocus} />
        <HashtagHeader
          hashtag={hashtag}
          onBack={this.handleBack}
          doSearch={this.handleChangeHash}
        />
        <GlobalFeedTemplate
          filter="hashtag"
          sort={sort}
          hashtag={hashtag}
          feedList={feedList}
          onRef={this.setGlobalFeedRef}
          onChangeSortOption={this.handleChangeSort}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  hashtagSort: state.hashtagSort,
  getFilteredList: getFilteredList(state),
  streamBuildNotificationSeen: state.appstate.streamBuildNotificationSeen,
});

const mapDispatchToProps = { fetchStream, updateSort, setStreamBuildNotificationSeen };

export default connect(mapStateToProps, mapDispatchToProps)(Hashtag);
