import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Share } from 'react-native';

import { fetchProfile, setProfileLoading } from '../reducers/profile';
import { screenWrapper } from '../utils/styles';

import StoreTabs from '../components/templates/StoreTabs';
import { createStoreUrlFromPeerID } from '../utils/navigation';
import OBActionSheet from '../components/organism/ActionSheet';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';
class Store extends PureComponent {
  componentDidMount() {
    this.props.fetchProfile();
  }

  onRefresh = () => {
    const { fetchProfile, setProfileLoading } = this.props;
    setProfileLoading(true);
    fetchProfile();
  };

  setActionSheet = (component) => {
    this.actionSheet = component;
  };

  handlePress = () => {
    this.actionSheet.show();
  };

  handleChange = (index) => {
    const { profile, navigation } = this.props;
    switch (index) {
      case 0: {
        const navKey = navigation.state.key;
        navigation.navigate({
          routeName: 'CreateListing',
          params: { navKey },
        });
        break;
      }
      case 1:
        navigation.navigate('NewFeed');
        break;
      case 2:
        Share.share({
          message: createStoreUrlFromPeerID(profile.data.peerID),
          title: '',
        });
        break;
      default:
        break;
    }
  };

  handleGoBack = () => this.props.navigation.goBack()

  toListingDetails = (params) => {
    eventTracker.trackEvent('UserProfile-ViewedUserListing');
    this.props.navigation.push('Listing', params);
  }

  render() {
    const { profile, navigation, loading } = this.props;
    return (
      <View style={screenWrapper.wrapper}>
        <StoreTabs
          profile={profile}
          toListingDetails={this.toListingDetails}
          onRefresh={this.onRefresh}
          key={loading}
          navigation={navigation}
          onMore={this.handlePress}
        />
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={[I18n.t('screens.store.Create_Listing'), I18n.t('screens.store.Create_Post'), I18n.t('screens.store.Share_to'), I18n.t('screens.store.Cancel')]}
          cancelButtonIndex={3}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile,
  loading: state.profile.loading,
});

const mapDispatchToProps = { fetchProfile, setProfileLoading };

export default connect(mapStateToProps, mapDispatchToProps)(Store);
