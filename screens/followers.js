import React, { PureComponent } from 'react';
import { FlatList, View, Text, ActivityIndicator } from 'react-native';
import * as _ from 'lodash';
import { connect } from 'react-redux';

import { getUserPeerID } from '../selectors/profile';
import { getFollowers } from '../api/follow';
import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import FriendItem from '../components/molecules/FriendItem';

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

class FollowersScreen extends PureComponent {
  state = {
    followers: [],
    loaded: false,
  };

  UNSAFE_componentWillMount() {
    const { navigation, myPeerID } = this.props;
    const peerID = navigation.getParam('peerID') || myPeerID;
    getFollowers(peerID).then((result) => {
      if (!Array.isArray(result)) {
        this.setState({ followers: [], loaded: true });
      } else {
        this.setState({ followers: result, loaded: true });
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
    const { followers, loaded } = this.state;
    const { navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    const name = navigation.getParam('name');

    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.onLeft} title={I18n.t('screens.followers.followers')}  />
        {!loaded && this.renderLoadingState()}
        {followers.length > 0 ? (
          <FlatList
            data={followers}
            keyExtractor={this.keyExtractor}
            renderItem={this.renderItem}
          />
        ) : (
          loaded && (
            <Text style={styles.placeholderText}>
              {peerID ? `${name} doesn't have any followers` : 'You don\'t have any followers'}
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

export default connect(mapStateToProps)(FollowersScreen);
