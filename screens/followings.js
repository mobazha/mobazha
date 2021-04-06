import React, { PureComponent } from 'react';
import { FlatList, View, Text, ActivityIndicator } from 'react-native';
import * as _ from 'lodash';
import { connect } from 'react-redux';

import { getUserPeerID } from '../selectors/profile';
import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import FriendItem from '../components/molecules/FriendItem';
import { getFollowings } from '../api/follow';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: '#fff',
  },
  placeholderText: {
    color: '#8A8A8A',
    textAlign: 'center',
    marginTop: 22,
  },
  activityIndicator: {
    paddingBottom: 10,
    marginTop: 30,
  },
};

class FollowingsScreen extends PureComponent {
  state = {
    followings: [],
    loaded: false,
  };

  UNSAFE_componentWillMount() {
    const { navigation, myPeerID } = this.props;

    const peerID = navigation.getParam('peerID') || myPeerID;
    getFollowings(peerID).then((result) => {
      if (!Array.isArray(result)) {
        this.setState({ followings: [], loaded: true });
      } else {
        this.setState({ followings: result, loaded: true });
      }
    });
  }

  onLeft = () => {
    this.props.navigation.goBack();
  };

  handlePress = (peerID) => {
    this.props.navigation.push('ExternalStore', { peerID });
  };

  keyExtractor = (item, index) => `peer_item_${index}`

  renderItem = ({ item }) => {
    const { navigation } = this.props;
    return (
      <FriendItem
        peerID={item}
        key={item}
        onPress={this.handlePress.bind(this, item)}
        navigation={navigation}
        showFollowButton
        fromFollowing
      />
    );
  };

  renderLoadingState = () => (
    <ActivityIndicator style={styles.activityIndicator} size="large" color="#8a8a8f" />
  );

  render() {
    const { followings, loaded } = this.state;
    const { navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    const name = navigation.getParam('name');
    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.onLeft} title={I18n.t('screens.followings.following')} />
        {!loaded && this.renderLoadingState()}
        {followings.length > 0 ? (
          <FlatList
            data={followings}
            keyExtractor={this.keyExtractor}
            renderItem={this.renderItem}
          />
        ) : (
          loaded && (
            <Text style={styles.placeholderText}>
              {peerID ? `${name} isn't following anyone` : 'You are not following anyone'}
            </Text>
          )
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  myPeerID: getUserPeerID(state),
});

export default connect(mapStateToProps)(FollowingsScreen);
