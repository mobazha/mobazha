/* eslint-disable no-lonely-if */
import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { ImageBackground, TouchableWithoutFeedback, View, Text, Dimensions } from 'react-native';
import ImagePicker from 'react-native-image-crop-picker';
import ActionSheet from 'react-native-actionsheet';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { getLocalHeaderImageSource } from '../../utils/files';
import { uploadImage } from '../../api/images';
import { foregroundColor, borderColor } from '../commonColors';
import AvatarImage from '../atoms/AvatarImage';
import { navHeightStyle } from '../../utils/navbar';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const styles = {
  wrapper: {},
  headerImage: {
    height: SCREEN_WIDTH / 3 + navHeightStyle.height,
    borderStyle: 'solid',
    borderBottom: 1,
    borderColor,
  },
  button: {
    width: '100%',
    height: '100%',
    backgroundColor: '#5d5d5d8c',
    alignItems: 'center',
    justifyContent: 'center',
  },
  buttonText: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#242424',
  },
  bottomSpacer: {
    height: 36,
  },
  avatarWrapper: {
    position: 'absolute',
    bottom: 0,
    alignItems: 'center',
    width: '100%',
    paddingBottom: 4,
  },
  avatarImgWrapper: {
  },
  avatarImage: {
    width: 64,
    height: 64,
  },
};

class ProfileImages extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      avatarHashes: props.avatarHashes,
      headerHashes: props.headerHashes,
    };
  }

  onSelectHeader = (image) => {
    const { username, password, onChange } = this.props;
    uploadImage(username, password, {
      filename: 'headerImage',
      image: image.data,
    }).then((imageHashes) => {
      eventTracker.trackEvent('EditProfile-ChangedCoverPhoto');
      this.setState({ headerHashes: imageHashes[0].hashes }, () => onChange(this.state));
    });
  }

  onSelectAvatar = (image) => {
    const { username, password, onChange } = this.props;
    uploadImage(username, password, {
      filename: 'avatarImage',
      image: image.data,
    }).then((imageHashes) => {
      eventTracker.trackEvent('EditProfile-ChangedUserAvatar');
      this.setState({ avatarHashes: imageHashes[0].hashes }, () => onChange(this.state));
    });
  }

  setActionSheet = (ref) => {
    this.actionSheet = ref;
  }

  setHeader = () => {
    this.isAvatarEditing = false;
    this.actionSheet.show();
  }

  setAvatar = () => {
    this.isAvatarEditing = true;
    this.actionSheet.show();
  }

  handleActionSheet = (index) => {
    const pickerOptions = {
      width: 500,
      height: this.isAvatarEditing ? 500 : 500 * ((SCREEN_WIDTH / 3 + navHeightStyle.height) / SCREEN_WIDTH),
      compressImageQuality: 1,
      includeBase64: true,
      cropping: true,
    };

    const callback = this.isAvatarEditing ? this.onSelectAvatar : this.onSelectHeader;

    if (index === 0) {
      ImagePicker.openCamera(pickerOptions).then(callback);
    } else if (index === 1) {
      ImagePicker.openPicker(pickerOptions).then(callback);
    }
  }

  render() {
    const { avatarHashes, headerHashes } = this.state;
    return (
      <View style={styles.wrapper}>
        <ImageBackground
          source={getLocalHeaderImageSource(_.get(headerHashes, 'large'))}
          style={styles.headerImage}
          resizeMode="cover"
        >
          <TouchableWithoutFeedback onPress={this.setHeader}>
            <View style={styles.button}>
              <Ionicons name="md-camera" size={32} color={foregroundColor} />
            </View>
          </TouchableWithoutFeedback>
        </ImageBackground>
        <View style={styles.bottomSpacer} />
        <View style={styles.avatarWrapper}>
          <View style={styles.avatarImgWrapper}>
            <AvatarImage
              style={styles.avatarImage}
              thumbnail={_.get(avatarHashes, 'tiny')}
              onPress={this.setAvatar}
              showCamera
              showLocal
            />
          </View>
        </View>
        <ActionSheet
          ref={this.setActionSheet}
          onPress={this.handleActionSheet}
          options={[I18n.t('components.organism.ProfileImages.take_photo'), I18n.t('components.organism.ProfileImages.choose_from_gallery'), I18n.t('components.organism.ProfileImages.cancel')]}
          cancelButtonIndex={2}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
});

export default connect(mapStateToProps)(ProfileImages);
