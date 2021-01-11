import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { withNavigation } from 'react-navigation';
import { KeyboardAvoidingView } from 'react-native';

import FeedDetailTemplate from '../components/templates/FeedDetail';
import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import StoreMoreMenu from '../components/molecules/StoreMoreMenu';

import { fetchFeedItems } from '../reducers/stream';
import { getActivity } from '../selectors/stream';
import { keyboardAvoidingViewSharedProps } from '../utils/keyboard';

import {I18n} from '../langs/I18n';

const wrapperStyle = { flex: 1, backgroundColor: 'white' };

class FeedDetail extends PureComponent {
  componentDidMount() {
    const { navigation, fetchFeedItems } = this.props;
    const activityId = navigation.getParam('activityId');
    fetchFeedItems([activityId]);
  }

  onLeft = () => {
    const { navigation } = this.props;
    const screenKey = navigation.getParam('screenKey');
    if (screenKey) {
      navigation.pop(2);
    } else {
      navigation.goBack();
    }
  }

  setFeedTemplate = (ref) => {
    this.feedTemplate = ref && ref.wrappedInstance;
  }

  handleShowActionSheet = () => {
    this.feedTemplate.handleShowActionSheet();
  };

  render() {
    return (
      <KeyboardAvoidingView style={wrapperStyle} {...keyboardAvoidingViewSharedProps}>
        <Header
          left={<NavBackButton />}
          onLeft={this.onLeft}
          title=""
          right={<StoreMoreMenu onMore={this.handleShowActionSheet} black />}
        />
        <FeedDetailTemplate onRef={this.setFeedTemplate} withRef />
      </KeyboardAvoidingView>
    );
  }
}

const mapStateToProps = state => ({
  getActivity: getActivity(state),
});

const mapDispatchToProps = { fetchFeedItems };

export default withNavigation(
  connect(mapStateToProps, mapDispatchToProps)(FeedDetail),
);
