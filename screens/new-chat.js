import React, { PureComponent } from 'react';
import { FlatList, View, Text, Platform, Dimensions, ActivityIndicator } from 'react-native';
import * as _ from 'lodash';
import { connect } from 'react-redux';
import Ionicons from 'react-native-vector-icons/Ionicons';

import PeerItem from '../components/molecules/PeerItem';
import { chatStyles } from '../utils/styles';
import NewChartHeader from '../components/molecules/NewChartHeader';
import ChartSearchField from '../components/molecules/ChatSearchField';
import { formLabelColor, secondaryTextColor } from '../components/commonColors';
import { setKeyword } from '../reducers/appstate';
import { appendUserSearch } from '../reducers/search';

import {I18n} from '../langs/I18n';

const { height } = Dimensions.get('window');

const styles = {
  search: {
    marginHorizontal: 12,
    marginVertical: 9,
  },
  emptyWrapper: {
    height: height * 0.6,
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyText: {
    marginTop: 10,
    fontSize: 15,
    textAlign: 'center',
    color: formLabelColor,
  },
  activityIndicator: {
    paddingBottom: 10,
    marginTop: 10,
  },
};

class NewChat extends PureComponent {
  state = {
    peerID: '',
    showSearchResult: false,
  };

  onChangeText = (text) => {
    this.setState({ peerID: text });
    if (_.isEmpty(text)) {
      this.setState({ showSearchResult: false });
    }
  };

  onPress = peerID => () => {
    this.props.navigation.navigate('ChatDetail', { peerID, subject: '' });
  };

  onLeft = () => {
    this.props.navigation.goBack();
  };

  resetKeyword = () => {
    this.setState({ showSearchResult: false });
  }

  handleSearch = () => {
    const { peerID } = this.state;
    if (_.isEmpty(peerID)) {
      return;
    }

    const { setKeyword } = this.props;
    this.setState({ showSearchResult: true });
    setKeyword(peerID);
  };

  renderItemWithFollowButton = ({ item }) => (
    <PeerItem
      showFollowButton
      peerID={item}
      key={item}
      onPress={this.onPress(item)}
    />
  );

  renderItem = ({ item }) => (
    <PeerItem
      showMessageButton
      peerID={item.peerID}
      key={item.peerID}
      onPress={this.onPress(item.peerID)}
    />
  );

  renderSearchField = () => {
    const { peerID } = this.state;
    return (
      <ChartSearchField
        style={styles.search}
        onChange={this.onChangeText}
        value={peerID}
        doSearch={this.handleSearch}
        placeholder={I18n.t('screens.newChat.Search')}
        hasBorder
        autoFocus
        resetKeyword={this.resetKeyword}
      />
    );
  }

  renderEmptyState = (icon, text = I18n.t('screens.newChat.Search_user')) => {
    const { loading } = this.props;
    return (
      <View style={styles.emptyWrapper}>
        {loading ?
          <ActivityIndicator size="large" />
          : icon &&
            <Ionicons
              name={Platform.OS === 'ios' ? `ios-${icon}` : `md-${icon}`}
              size={80}
              color={formLabelColor}
            />
        }
        <Text style={styles.emptyText}>
          {loading ? 'Loading' : text}
        </Text>
      </View>
    );
  };

  renderListFooter = () => {
    const { loading, user_more_page } = this.props;
    return (
      !loading && user_more_page && (
        <ActivityIndicator
          style={styles.activityIndicator}
          size="large"
          color={secondaryTextColor}
        />
      )
    );
  };

  render() {
    const { followings, searchResults, appendUserSearch } = this.props;
    const { showSearchResult } = this.state;

    return (
      <View style={chatStyles.bgWrapper}>
        <NewChartHeader
          onBack={this.onLeft}
          searchComponent={this.renderSearchField()}
        />
        {showSearchResult && (
          <FlatList
            data={searchResults.map(result => _.get(result, 'data.peerID'))}
            keyExtractor={(item, index) => `peer_item_${index}`}
            renderItem={this.renderItemWithFollowButton}
            ListEmptyComponent={this.renderEmptyState('search', 'No results found')}
            ListFooterComponent={this.renderListFooter()}
            onEndReached={appendUserSearch}
          />
        )}
        {!showSearchResult && (
          <FlatList
            data={followings}
            keyExtractor={(item, index) => `peer_item_${index}`}
            renderItem={this.renderItem}
            ListEmptyComponent={this.renderEmptyState('person')}
          />
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  searchResults: state.search.user_search_result,
  followings: state.follow.followings,
  loading: state.search.loading_user,
  user_more_page: state.search.user_more_page,
});

const mapDispatchToProps = {
  setKeyword,
  appendUserSearch,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(NewChat);
