import React from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';
import { NavigationActions } from 'react-navigation';
import * as _ from 'lodash';

import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import StoreModeratorsList from '../components/templates/StoreModeratorList';
import { screenWrapper } from '../utils/styles';

import { fetchModerators } from '../reducers/moderators';
import { fetchProfile } from '../reducers/profile';

import {I18n} from '../langs/I18n';
class StoreModerators extends React.PureComponent {
  UNSAFE_componentWillMount() {
    const { fetchModerators } = this.props;
    fetchModerators();
  }

  componentDidUpdate(prevProps) {
    const { storeModerators } = this.props;
    if (!_.isEqual(prevProps.storeModerators, storeModerators)) {
      this.fetchModeratorProfiles();
    }
  }

  fetchModeratorProfiles = () => {
    const { fetchProfile, storeModerators } = this.props;

    if (storeModerators) {
      for (let i = 0; i < storeModerators.length; i += 1) {
        fetchProfile({ peerID: storeModerators[i], getLoaded: true });
      }
    }
  }

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.dispatch(NavigationActions.back());
          }}
          title="Manage Moderators"
        />
        <StoreModeratorsList />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  storeModerators: state.settings.storeModerators || [],
});

const mapDispatchToProps = {
  fetchProfile,
  fetchModerators,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(StoreModerators);
