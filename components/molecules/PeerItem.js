import React, { Component } from 'react';
import { withNavigation } from 'react-navigation';
import * as _ from 'lodash';
import { connect } from 'react-redux';
import { fetchProfile } from '../../reducers/profile';
import { getDefaultProfileFromPeerId } from '../../utils/profile';

import FriendItem from './FriendItem';

class PeerItem extends Component {
  static getDerivedStateFromProps(props) {
    const { peerID, profiles } = props;
    return { profile: profiles && profiles[peerID] };
  }

  state = {
    profile: null,
  };

  shouldComponentUpdate(nextProps, nextState) {
    const { profile } = this.state;
    const nextProfile = nextState.profile;
    if (!profile && nextProfile) {
      return true;
    } else {
      return false;
    }
  }

  render() {
    const { peerID, ...props } = this.props;
    const profile = this.state.profile || getDefaultProfileFromPeerId(peerID);

    // return <FriendItem item={profile} {...props} />;
    return <FriendItem {...props} />;
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
});

const mapDispatchToProps = {
  fetchProfile,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(PeerItem));
