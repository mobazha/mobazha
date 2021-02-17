import React from 'react';
import { connect } from 'react-redux';
import { Animated, View, TouchableWithoutFeedback, Text } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';
import MaterialIcons from 'react-native-vector-icons/MaterialIcons';
import { NavigationActions, StackActions } from 'react-navigation';

import { greenColor, greenTintColor, sectionTitleTextColor } from '../../commonColors';

import { resetData } from '../../../reducers/createListing';
import { hidePanel } from '../../../reducers/appstate';

import { eventTracker } from '../../../utils/EventTracker';

import PanelViewBase from './PanelViewBase';

import {I18n} from '../../../langs/I18n';

const styles = {
  menuItem: {
    flex: 1,
    flexDirection: 'column',
    alignItems: 'center',
  },
  iconWrapper: {
    alignItems: 'center',
    justifyContent: 'center',
    width: 50,
    height: 50,
    borderRadius: 25,
    backgroundColor: greenTintColor,
  },
  menuItemTitle: {
    fontSize: 15,
    color: sectionTitleTextColor,
    marginTop: 10,
    textAlign: 'center',
  },
  featherIcons: {
    paddingLeft: 2,
    color: greenColor,
  },
};

const SELL_ICON = <Feather name="tag" size={24} style={styles.featherIcons} />;
const PAY_ICON = <MaterialIcons name="account-balance-wallet" size={24} style={styles.featherIcons} />;
const CHAT_ICON = <Feather name="message-circle" size={24} style={styles.featherIcons} />;
const POST_ICON = <Feather name="edit" size={24} style={styles.featherIcons} />;

const MENU_ITEMS = [
  { title: I18n.t('components.organism.PanelView.PlusPanelView.Sell'), icon: SELL_ICON },
  { title: I18n.t('components.organism.PanelView.PlusPanelView.Post'), icon: POST_ICON },
  { title: I18n.t('components.organism.PanelView.PlusPanelView.Chat'), icon: CHAT_ICON },
  { title: I18n.t('components.organism.PanelView.PlusPanelView.Pay'), icon: PAY_ICON },
];

class PlusPanelView extends React.Component {
  gridBaseRef = React.createRef();

  handlePress = idx => () => {
    this.gridBaseRef.current.handleHideGrid();
    const { navigator, resetData } = this.props;
    switch (idx) {
      case 0:
        resetData();
        eventTracker.trackEvent('MainNavigationTap-CreateListing');
        navigator.dispatch(StackActions.push({ routeName: 'CreateListing' }));
        break;
      case 1:
        eventTracker.trackEvent('MainNavigationTap-CreatePost');
        navigator.dispatch(StackActions.push({ routeName: 'NewFeed' }));
        break;
      case 2:
        eventTracker.trackEvent('MainNavigationTap-SendMessage');
        navigator.dispatch(StackActions.push({ routeName: 'NewChat' }));
        break;
      case 3:
        eventTracker.trackEvent('MainNavigationTap-SendMoney');
        navigator.dispatch(StackActions.push({ routeName: 'SendMoney' }));
        break;
      default:
        break;
    }
  }

  renderMenuItems = () => MENU_ITEMS.map(({ title, icon }, idx) => (
    <TouchableWithoutFeedback onPress={this.handlePress(idx).bind(this)} key={`menuItem_${idx}`}>
      <View style={styles.menuItem}>
        <View style={styles.iconWrapper}>
          {icon}
        </View>
        <Text style={styles.menuItemTitle}>{title}</Text>
      </View>
    </TouchableWithoutFeedback>
  ));

  render() {
    const { currentPanel, hidePanel } = this.props;
    return (
      <PanelViewBase
        title={I18n.t('components.organism.PanelView.PlusPanelView.Choose_action')}
        ref={this.gridBaseRef}
        isShowingPanel={currentPanel.type === 'plus'}
        hidePanel={hidePanel}
      >
        {this.renderMenuItems()}
      </PanelViewBase>
    );
  }
}

const mapStateToProps = state => ({
  currentPanel: state.appstate.currentPanel || {},
});

const mapDispatchToProps = {
  resetData,
  hidePanel,
};

export default connect(mapStateToProps, mapDispatchToProps)(PlusPanelView);
