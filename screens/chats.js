import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, RefreshControl, Text, Image } from 'react-native';
import { FlatList } from 'react-navigation';
import { isEmpty, filter, get, sortBy } from 'lodash';
import Feather from 'react-native-vector-icons/Feather';
import Foundation from 'react-native-vector-icons/Foundation';

import { fetchChats, setChatsLoading } from '../reducers/chat';

import HollowButton from '../components/atoms/HollowButton';
import ChatListItem from '../components/molecules/ChatListItem';
import TabHeader from '../components/organism/TabHeader';
// import Tabs from '../components/organism/Tabs';

import { getDefaultProfileFromPeerId } from '../utils/profile';

import { formLabelColor } from '../components/commonColors';
import NavPlusButton from '../components/atoms/NavPlusButton';
import { chatsMap } from '../selectors/chat';

import {I18n} from '../langs/I18n';

const styles = {
  placeholderWrapper: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  placeholderImage: {
    width: 50,
    height: 50,
    tintColor: '#8a8a8f',
    marginBottom: 10,
  },
  placeholderText: {
    width: 298.5,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
    paddingTop: 10,
  },
  bgWrapper: {
    backgroundColor: 'white',
    flex: 1,
  },
  buttonWrapper: {
    marginTop: 8,
  },
  emptyStateStyle: { flex: 1, justifyContent: 'center' },
};

// const chatTabs = [
//   {
//     value: 'conversations',
//     label: 'Conversations',
//   },
//   {
//     value: 'orders',
//     label: 'Orders',
//   },
// ];

class Chats extends PureComponent {
  state = {
    tab: 'conversations',
  };
  componentDidMount() {
    this.props.fetchChats();
  }

  onRefresh = () => {
    const { fetchChats, setChatsLoading } = this.props;
    setChatsLoading(true);
    fetchChats();
  };

  toNewChat = () => {
    this.props.navigation.navigate('NewChat');
  };

  updateChatFilter = (tab) => {
    this.setState({ tab });
  };

  filterChats = () => {
    const { chats } = this.props;
    const { tab } = this.state;
    const filteredData = filter(chats.data, (o) => {
      if (tab === 'conversations') {
        return get(o, 'subject', '') === '';
      } else {
        return get(o, 'subject', '') !== '';
      }
    });
    const sortedData = sortBy(filteredData, [
      o => (o.unread === 0 ? 1 : 0),
      o => -new Date(o.timestamp).getTime(),
    ]);
    return sortedData;
  };

  renderItem = ({ item, index }) => {
    const { profiles, navigation } = this.props;
    const profile = profiles && profiles[item.peerId];
    const filteredData = this.filterChats();
    return (
      <ChatListItem
        item={item}
        key={item.slug}
        profile={profile || getDefaultProfileFromPeerId(item.peerId)}
        isFirst={index === 0}
        isLast={index === filteredData.length - 1}
        navigation={navigation}
      />
    );
  };

  renderEmptyState = () => {
    const { tab } = this.state;
    return tab === 'conversations' ? (
      <View style={styles.placeholderWrapper}>
        <Feather name="message-circle" size={50} color="#8a8a8f" />
        <Text style={styles.placeholderText}>{I18n.t('screens.chats.start_conversation')}</Text>
        <View style={styles.buttonWrapper}>
          <HollowButton title={I18n.t('screens.chats.new_chat')} onPress={this.toNewChat} />
        </View>
      </View>
    ) : (
      <View style={styles.placeholderWrapper}>
        <Foundation name="price-tag" size={50} color={formLabelColor} />
        <Text style={styles.placeholderText}>{I18n.t('screens.chats.no_discussion')}</Text>
      </View>
    );
  };

  render() {
    const { chats, navigation } = this.props;
    const filteredData = this.filterChats();
    // const { tab } = this.state;

    return (
      <View style={styles.bgWrapper}>
        <TabHeader
          left={<NavPlusButton />}
          onLeft={this.toNewChat}
          title={I18n.t('screens.chats.chat')}
          navigation={navigation}
        />
        {/* <Tabs
          currentTab={tab}
          tabs={chatTabs}
          onChange={this.updateChatFilter}
          withBorder
        /> */}
        <FlatList
          ListEmptyComponent={this.renderEmptyState()}
          contentContainerStyle={isEmpty(filteredData) ? styles.emptyStateStyle : null}
          data={filteredData}
          keyExtractor={(item, index) => `chat_item_${index}`}
          renderItem={this.renderItem}
          refreshControl={<RefreshControl refreshing={chats.loading} onRefresh={this.onRefresh} />}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  ...chatsMap(state),
});

const mapDispatchToProps = {
  fetchChats,
  setChatsLoading,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Chats);
