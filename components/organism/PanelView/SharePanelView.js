import React from 'react';
import { connect } from 'react-redux';
import { View, TouchableWithoutFeedback, Text, Share } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';

import { greenColor, greenTintColor, sectionTitleTextColor } from '../../commonColors';

import { hidePanel } from '../../../reducers/appstate';

import PanelViewBase from './PanelViewBase';
import { createListingUrlFromPeerIDAndSlug } from '../../../utils/navigation';
import PostListingTemplate from '../../templates/SocialPostTemplate/PostListingTemplate';

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

const SOCIAL_ICON = <Feather name="users" size={24} style={styles.featherIcons} />;
const EXTERNAL_ICON = <Feather name="share-2" size={24} style={styles.featherIcons} />;

const MENU_ITEMS = [
  { title: I18n.t('components.organism.PanelView.SharePanelView.Social'), icon: SOCIAL_ICON },
  { title: I18n.t('components.organism.PanelView.SharePanelView.External'), icon: EXTERNAL_ICON },
];

class SharePanelView extends React.Component {
  state = {
    showModal: false,
  }

  panelBaseRef = React.createRef();

  handlePress = idx => () => {
    const { currentPanel } = this.props;
    const { panelData: listing } = currentPanel;
    const {
      slug,
      item: { title },
      vendorID: { peerID: storeId },
    } = listing;

    this.panelBaseRef.current.handleHideGrid();

    switch (idx) {
      case 0:
        this.setState({ showModal: true });
        break;
      case 1:
        Share.share({
          message: createListingUrlFromPeerIDAndSlug(storeId, slug),
          title,
        });
        break;
      default:
        break;
    }
  }

  handleHideModal = () => {
    this.setState({ showModal: false });
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
    const { panelData: listing } = currentPanel;
    const { showModal } = this.state;
    return (
      <React.Fragment>
        <PanelViewBase
          ref={this.panelBaseRef}
          title={I18n.t('components.organism.PanelView.SharePanelView.Share_to')}
          isShowingPanel={currentPanel.type === 'share'}
          hidePanel={hidePanel}
        >
          {this.renderMenuItems()}
        </PanelViewBase>
        <PostListingTemplate
          showModal={showModal}
          onHideModal={this.handleHideModal}
          listing={listing}
        />
      </React.Fragment>
    );
  }
}

const mapStateToProps = state => ({
  currentPanel: state.appstate.currentPanel || {},
});

const mapDispatchToProps = {
  hidePanel,
};

export default connect(mapStateToProps, mapDispatchToProps)(SharePanelView);
