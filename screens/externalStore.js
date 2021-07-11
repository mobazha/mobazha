import React, { Component } from 'react';
import { connect } from 'react-redux';
import { View, Text, Share, ActivityIndicator } from 'react-native';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Feather from 'react-native-vector-icons/Feather';
import deepEqual from 'deep-equal';

import StoreTabs from '../components/templates/StoreTabs';
import ExternalStoreHeader from '../components/molecules/ExternalStoreHeader';
import HollowButton from '../components/atoms/HollowButton';
import { wrapperStyles } from '../utils/styles';

import { reportProfile } from '../api/profile';
import { fetchProfile, setProfileLoading } from '../reducers/profile';
import { blockNode, unblockNode } from '../reducers/settings';
import { createStoreUrlFromPeerID } from '../utils/navigation';
import OBActionSheet from '../components/organism/ActionSheet';
import ReportTemplate from '../components/templates/ReportTemplate';
import { eventTracker } from '../utils/EventTracker';
import { formLabelColor } from '../components/commonColors';

import {I18n} from '../langs/I18n';

const styles = {
  emptyWrapper: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyContent: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyText: {
    marginTop: 12,
    marginBottom: 10,
    width: 311,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 120,
    bottom: 60,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class ExternalStore extends Component {
  static getDerivedStateFromProps(props) {
    const peerID = props.navigation.getParam('peerID');
    const { profiles } = props;
    return { profile: profiles && profiles[peerID] };
  }

  state = { profile: null, reported: false, isMyStore: false };

  componentDidMount() {
    if (!this.isBlocked()) {
      this.loadProfile();
    }
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { profile, reported, isDue } = this.state;
    if ((!profile && nextState.profile)
      || !deepEqual(this.props.blockedNodes, nextProps.blockedNodes)
      || (isDue !== nextState.isDue)
    ) {
      return true;
    } else if (reported !== nextState.reported) {
      return true;
    } else {
      return false;
    }
  }

  setActionSheet = (component) => { this.actionSheet = component; };

  setReportTemplateRef = (component) => { this.reportTemplate = component; }

  loadProfile = () => {
    const { fetchProfile, navigation } = this.props;
    const peerID = navigation.getParam('peerID');

    this.setState({ isDue: false });
    fetchProfile({ peerID });
    setTimeout(() => this.setState({ isDue: true }), 5000);
  }

  reportUser = (reason, option) => {
    const { navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    reportProfile(peerID, `${option} - ${reason}`);
    this.showTooltip();
  }

  showTooltip() {
    this.setState({ reported: true }, () => {
      setTimeout(() => { this.setState({ reported: false }); }, 2000);
    });
  }

  handlePressMore = () => {
    this.actionSheet.show();
  }

  handleExternalStoreAction = (index) => {
    const peerID = this.props.navigation.getParam('peerID');
    switch (index) {
      case 0: {
        if (this.reportTemplate) {
          this.reportTemplate.startStep();
        }
        break;
      }
      case 1:
        eventTracker.trackEvent('UserProfile-BlockedUser');
        this.blockNode();
        break;
      case 2:
        eventTracker.trackEvent('UserProfile-SharedUser');
        Share.share({
          message: createStoreUrlFromPeerID(peerID),
          title: '',
        });
        break;
      default:
        break;
    }
  };

  handleLocalStoreAction = (index) => {
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
  }

  unblockNode = () => {
    const peerID = this.props.navigation.getParam('peerID');
    const { fetchProfile, unblockNode } = this.props;
    unblockNode(peerID);
    fetchProfile({ peerID });
  };

  isBlocked = () => {
    const { blockedNodes, navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    return blockedNodes.includes(peerID);
  };

  blockNode = () => {
    const peerID = this.props.navigation.getParam('peerID');
    this.props.blockNode(peerID);
  }

  toListingDetails = (params) => {
    eventTracker.trackEvent('UserProfile-ViewedUserListing');
    this.props.navigation.push('Listing', params);
  }

  handleGoBack = () => {
    this.props.navigation.goBack();
  }

  renderBlockedContent = () => {
    const { profile } = this.state;
    return (
      <View style={styles.emptyWrapper}>
        <ExternalStoreHeader onBack={this.handleGoBack} profile={profile} isBlocked onMore={this.handlePressMore} />
        <View style={styles.emptyContent}>
          <Ionicons size={50} name="md-eye-off" color="#8a8a8f" />
          <Text style={styles.emptyText}>{I18n.t('screens.externalStore.unblock_user')}</Text>
          <HollowButton title={I18n.t('screens.externalStore.unblock')} onPress={this.unblockNode} />
        </View>
      </View>
    );
  }

  renderLoadingState = () => (
    <View style={styles.emptyWrapper}>
      <ExternalStoreHeader onBack={this.handleGoBack} />
      <View style={styles.emptyContent}>
        <ActivityIndicator size= "large" color={formLabelColor} />
        <Text style={styles.emptyText}>{I18n.t('screens.externalStore.loading')}</Text>
      </View>
    </View>
  );

  renderEmptyContent = () => (
    <View style={styles.emptyWrapper}>
      <ExternalStoreHeader onBack={this.handleGoBack} />
      <View style={styles.emptyContent}>
        <Feather size={50} name="user-x" color="#8a8a8f" />
        <Text style={styles.emptyText}>{I18n.t('screens.externalStore.failed_load')}</Text>
        <HollowButton title={I18n.t('screens.externalStore.retry')} onPress={this.loadProfile} />
      </View>
    </View>
  );

  render() {
    const { loading, navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    const { profile, reported, isMyStore, isDue } = this.state;
    const isBlocked = this.isBlocked();
    return (
      <View style={wrapperStyles.bottomWrapperExternalStore} key={`profile_${peerID}`}>
        {isBlocked ? this.renderBlockedContent() : profile ? (
          <StoreTabs
            profile={{ data: profile, loading }}
            peerID={peerID}
            toListingDetails={this.toListingDetails}
            externalStore
            navigation={navigation}
            onMore={this.handlePressMore}
          />
        ) : isDue ? this.renderEmptyContent() : this.renderLoadingState()}
        {reported && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>
            {I18n.t('screens.externalStore.reported')}
            </Text>
          </View>
        )}
        {isMyStore ? (
          <OBActionSheet
            ref={this.setActionSheet}
            onPress={this.handleLocalStoreAction}
            options={[I18n.t('screens.externalStore.Create_Listing'), I18n.t('screens.externalStore.Create_Post'), I18n.t('screens.externalStore.Share_to'), I18n.t('screens.externalStore.Cancel')]}
            cancelButtonIndex={3}
          />
        ) : (
          <OBActionSheet
            ref={this.setActionSheet}
            onPress={this.handleExternalStoreAction}
            options={[I18n.t('screens.externalStore.Report_user'), I18n.t('screens.externalStore.Block_user'), I18n.t('screens.externalStore.Share_to'), I18n.t('screens.externalStore.Cancel')]}
            cancelButtonIndex={3}
          />
        )}
        <ReportTemplate
          ref={this.setReportTemplateRef}
          submit={this.reportUser}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  userProfile: state.profile.data,
  profiles: state.profiles,
  loading: state.profile.loading,
  blockedNodes: state.settings.blockedNodes,
});

const mapDispatchToProps = {
  fetchProfile,
  setProfileLoading,
  blockNode,
  unblockNode,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ExternalStore);
