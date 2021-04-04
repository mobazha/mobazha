import React from 'react';
import { connect } from 'react-redux';
import { withNavigation } from 'react-navigation';
import { View, Text, FlatList, Dimensions, TouchableWithoutFeedback } from 'react-native';
import FastImage from 'react-native-fast-image';
import * as _ from 'lodash';
import decode from 'unescape';

import { chatStyles } from '../../utils/styles';
import OBFastImage from '../atoms/OBFastImage';
import {
  getListingIfExists,
  getListingLinkFromContent,
  fetchListingLink,
  handlePressListing,
  renderContentWithListingPreview,
  handleDeeplinkPress,
  handleCrypoLink,
  handleHashtag,
  getFeedInfo,
  getStatusToDecode,
} from '../organism/ContentWithListingPreview';
import { borderColor, primaryTextColor, brandColor } from '../commonColors';

import { fetchListing } from '../../reducers/listings';
import { getActivity } from '../../selectors/stream';

import {I18n} from '../../langs/I18n';

const DEVICE_WIDTH = Dimensions.get('window').width;
const RIGHT_PADDING = chatStyles.avatarImage.marginLeft;
const LEFT_PADDING = chatStyles.avatarImage.width + chatStyles.avatarImage.marginLeft * 2;
const getImageItemWidth = count =>
  (DEVICE_WIDTH - RIGHT_PADDING - LEFT_PADDING - (count - 1) * RIGHT_PADDING / 2 - 50) / count;

const styles = {
  wrapper: {
    backgroundColor: 'transparent',
    flexDirection: 'column',
    borderWidth: 1,
    borderColor,
    padding: 13.6,
  },
  name: {
    fontSize: 13.6,
    fontWeight: '600',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#000000',
    marginBottom: 3,
  },
  text: {
    fontSize: 14.5,
    lineHeight: 17.3,
    color: primaryTextColor,
    marginBottom: 9.1,
  },
  imageGrid: {
    width: '100%',
  },
  linkText: {
    color: brandColor,
  },
};

class FeedPreview extends React.Component {
  static getDerivedStateFromProps(props) {
    const { peerID, profiles } = props;
    return { profile: profiles && profiles[peerID] };
  }

  constructor(props) {
    super(props);

    this.getListingIfExists = getListingIfExists.bind(this);
    this.getListingLinkFromContent = getListingLinkFromContent.bind(this);
    this.fetchListingLink = fetchListingLink.bind(this);
    this.handlePressListing = handlePressListing.bind(this);
    this.renderContentWithListingPreview = renderContentWithListingPreview.bind(this);
    this.handleDeeplinkPress = handleDeeplinkPress.bind(this);
    this.handleCrypoLink = handleCrypoLink.bind(this);
    this.handleHashtag = handleHashtag.bind(this);
    this.getFeedInfo = getFeedInfo.bind(this);
    this.getStatusToDecode = getStatusToDecode.bind(this);

    this.styles = styles;
  }

  state = {
    profile: null,
  };

  componentDidMount() {
    if (this.props.activityId) {
      this.fetchListingLink();
    }
  }

  componentDidUpdate(prevProps) {
    if (this.getStatusToDecode(prevProps) !== this.getStatusToDecode(this.props)) {
      this.fetchListingLink();
    }
  }

  setActionSheet = (component) => {
    this.actionSheet = component;
  };

  getImageSize = () => {
    const { activityId, getActivity } = this.props;
    const activityItem = getActivity(activityId);
    const postData = _.get(activityItem, 'object.data.post', {});
    const images = postData.images || [];
    return getImageItemWidth(Math.min(3, images.length));
  }

  handlePress = () => {
    const { activityId, navigation } = this.props;
    navigation.push('FeedDetail', {
      activityId, tab: 'comment', showKeyboard: false,
    });
  }

  handlePressMore = () => {
    this.actionSheet.show();
  }

  isFeedOwner = () => {
    const { peerID } = this.props;
    return !peerID;
  }

  keyExtractor = (item, index) => `item_${index}`;

  renderImage = ({ item: image, index }) => {
    const imageSize = this.getImageSize();
    const { medium } = image;
    const style = {
      marginLeft: index === 0 ? 0 : RIGHT_PADDING,
      width: imageSize,
      height: imageSize,
    };
    return (
      <OBFastImage style={style} hash={medium} resizeMode={FastImage.resizeMode.cover} />
    );
  }

  render() {
    const { activityId, getActivity } = this.props;
    const item = getActivity(activityId);
    if (_.isEmpty(item)) {
      return false;
    }
    const postData = _.get(item, 'object.data.post', {});
    const { profile = {} } = this.state;
    const images = postData.images || [];
    const { name = 'Anonymous' } = profile;
    const height = this.getImageSize();

    return (
      <TouchableWithoutFeedback onPress={this.handlePress}>
        <View style={styles.wrapper}>
          <Text style={styles.name} numberOfLines={1}>
            {`${name || I18n.t('components.molecules.FeedItem.reposted')}`}
          </Text>
          {this.renderContentWithListingPreview(true)}
          {images.length > 0 && (
            <FlatList
              horizontal
              style={[styles.imageGrid, { height }]}
              data={_.take(images, 3)}
              keyExtractor={this.keyExtractor}
              renderItem={this.renderImage}
            />
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  getActivity: getActivity(state),
  listings: state.listings.data,
});

const mapDispatchToProps = {
  fetchListing,
};

export default withNavigation(connect(mapStateToProps, mapDispatchToProps)(FeedPreview));
