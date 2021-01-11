import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Text, View, KeyboardAvoidingView, Alert, Keyboard } from 'react-native';
import * as _ from 'lodash';

import Header from '../components/molecules/Header';
import { primaryTextColor, foregroundColor } from '../components/commonColors';
import NavBackButton from '../components/atoms/NavBackButton';
import AvatarImage from '../components/atoms/AvatarImage';
import PlaceholderStyleTextInput from '../components/atoms/PlaceholderStyleTextInput';
import NewsFeedFooter from '../components/templates/NewsFeedFooter';
import { chatStyles } from '../utils/styles';

import { postFeed } from '../reducers/feed';

import { eventTracker } from '../utils/EventTracker';
import { keyboardAvoidingViewSharedProps } from '../utils/keyboard';

import {I18n} from '../langs/I18n';

const MAJOR_PADDING = chatStyles.avatarImage.marginLeft;
const MAX_CHAR = 280;

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
  },
  contentWrapper: {
    flex: 1,
    paddingBottom: 10,
  },
  contentContainer: {
    flex: 1,
    backgroundColor: foregroundColor,
    flexDirection: 'row',
    paddingTop: 7,
  },
  nonAvatarContainer: {
    marginLeft: 8,
    marginTop: 8,
    flexDirection: 'column',
    flex: 1,
  },
  input: {
    flex: 1,
    width: '100%',
    fontSize: 19,
    fontWeight: '400',
    fontStyle: 'normal',
    color: primaryTextColor,
    backgroundColor: 'white',
    textAlignVertical: 'top',
    paddingTop: 10,
    paddingRight: MAJOR_PADDING,
    lineHeight: 24,
  },
  text: {
    backgroundColor: 'white',
    padding: MAJOR_PADDING,
    textAlign: 'right',
    color: 'rgb(141, 141, 141)',
    fontSize: 12,
  },
  boldCharacterCount: {
    fontWeight: 'bold',
  },
  avatarImage: {
    width: 45,
    height: 45,
    marginVertical: 6,
    marginLeft: 13,
  },
};

class NewFeed extends PureComponent {
  state = { text: '', images: [], loading: false };

  componentDidMount() {
    this.focusListener = this.props.navigation.addListener('didFocus', this.setInputFocus);
  }

  componentWillUnmount() {
    this.focusListener.remove();
  }

  onPostFeedSuccess = () => {
    eventTracker.trackEvent('Social-PostCreated');
    this.goBack();
  }

  onPostFeedFailure = () => {
    this.setState({ loading: false });
    Alert.alert(I18n.t('screens.newFeed.Create_failed'), I18n.t('screens.newFeed.unknown_error_create'));
  }

  setInputFocus = () => {
    this.inputRef.focus();
  }

  setInputRef = (ref) => {
    this.inputRef = ref;
  }

  goBack = () => {
    const { navigation } = this.props;
    Keyboard.dismiss();
    navigation.pop();
  }

  handlePost = () => {
    const { text, images } = this.state;
    const filteredImages = images.filter(item => item.hashes != null).map((item) => {
      const { filename, hashes } = item;
      return { ...hashes, filename };
    });

    const { postFeed } = this.props;
    this.setState({ loading: true });
    const payload = {
      status: text,
      images: filteredImages,
      success: this.onPostFeedSuccess,
      failure: this.onPostFeedFailure,
    };
    postFeed(payload);
  };

  handleChangeImages = (images) => {
    eventTracker.trackEvent('Social-AttachedPhotos');
    this.setState({ images });
  }

  isPostReady = () => {
    const { text, images } = this.state;
    const filteredImages = images.filter(item => item.hashes != null).map((item) => {
      const { filename, hashes } = item;
      return { ...hashes, filename };
    });

    return _.isEmpty(text) && filteredImages.length === 0;
  }

  renderCharacterCount = () => {
    const { text } = this.state;
    if (text.length > 0) {
      return (
        <Text style={styles.text}>
          <Text style={styles.boldCharacterCount}>{`${MAX_CHAR - text.length}`}</Text>{I18n.t('screens.newFeed.char_left')}
        </Text>
      );
    }
    return false;
  }

  render() {
    const { profile } = this.props;
    const { text, loading } = this.state;
    return (
      <KeyboardAvoidingView style={styles.wrapper} {...keyboardAvoidingViewSharedProps} >
        <Header
          left={<NavBackButton />}
          onLeft={this.goBack}
          right={this.renderCharacterCount()}
        />
        <View style={styles.contentWrapper}>
          <View style={styles.contentContainer}>
            <AvatarImage
              style={styles.avatarImage}
              thumbnail={_.get(profile, 'avatarHashes.tiny')}
            />
            <View style={styles.nonAvatarContainer}>
              <PlaceholderStyleTextInput
                inputRef={this.setInputRef}
                placeholder={I18n.t('screens.newFeed.what_going_on')}
                style={styles.input}
                onChangeText={value => this.setState({ text: value })}
                value={text}
                underlineColorAndroid="transparent"
                multiline
                maxLength={MAX_CHAR}
              />
            </View>
          </View>
          <NewsFeedFooter
            onChange={this.handleChangeImages}
            onPost={this.handlePost}
            buttonDisabled={this.isPostReady()}
            loading={loading}
          />
        </View>
      </KeyboardAvoidingView>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
});

const mapDispatchToProps = { postFeed };

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(NewFeed);
