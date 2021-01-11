import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, FlatList, TouchableWithoutFeedback } from 'react-native';
import { withNavigation } from 'react-navigation';

import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import Moderator from '../components/organism/Moderator';

import { screenWrapper } from '../utils/styles';
import { secondaryTextColor, borderColor, primaryTextColor } from '../components/commonColors';
import { fetchModerators } from '../reducers/moderators';

import {I18n} from '../langs/I18n';

const styles = {
  loadingWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 20,
  },
  loadingMessage: {
    paddingHorizontal: 17,
    fontSize: 15,
    color: secondaryTextColor,
  },
  title: {
    paddingHorizontal: 17,
    marginTop: 20,
    marginBottom: 16,
    fontSize: 16,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: -0.1,
    textAlign: 'left',
    color: primaryTextColor,
  },
  moderator: {
    padding: 17,
    borderTopWidth: 1,
    borderColor,
  },
  last: {
    borderBottomWidth: 1,
    paddingBottom: 17,
  },
};

class CheckoutModerators extends PureComponent {
  constructor(props) {
    super(props);
    props.fetchModerators();
  }

  getRenderableModerators() {
    const moderators = this.props.navigation.getParam('moderators');
    const { profiles, moderators: verifiedModerators } = this.props;
    const renderables = moderators.filter(peerID => profiles[peerID] && verifiedModerators.includes(peerID));
    return renderables;
  }

  handlePressModerator = (moderator) => {
    this.props.navigation.push('CheckoutModeratorDetails', { moderator });
  }

  renderItem = ({ item, index }) => {
    const moderators = this.getRenderableModerators();
    return (
      <TouchableWithoutFeedback onPress={() => this.handlePressModerator(item)}>
        <View style={[styles.moderator, index === moderators.length - 1 ? styles.last : {}]}>
          <Moderator peerID={item} />
        </View>
      </TouchableWithoutFeedback>
    );
  }

  render() {
    const moderators = this.getRenderableModerators();
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => this.props.navigation.goBack()}
        />
        <Text style={styles.title}>{I18n.t('screens.checkoutModerators.select_moderator')}</Text>
        {moderators.length > 0 ? (
          <FlatList
            data={moderators}
            renderItem={this.renderItem}
          />
        ) : (
          <View style={styles.loadingWrapper}>
            <Text style={styles.loadingMessage}>{I18n.t('screens.checkoutModerators.no_available')}</Text>
          </View>
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  moderators: state.moderators,
});

const mapDispatchToProps = {
  fetchModerators,
};
export default withNavigation(connect(mapStateToProps, mapDispatchToProps)(CheckoutModerators));
