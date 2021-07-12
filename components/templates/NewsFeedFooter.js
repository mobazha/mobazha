import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { FlatList, StyleSheet, View, Alert, Platform } from 'react-native';
import ImagePicker from 'react-native-image-crop-picker';
import ImageViewer from 'react-native-image-zoom-viewer';
import { hasIn, get } from 'lodash';

import ImagePick from '../molecules/ImagePick';
import { OBDarkModal } from '../templates/OBModal';
import ModalBackButton from '../atoms/ModalBackButton';
import ModalTrashButton from '../atoms/ModalTrashButton';
import ModalImageIndicator from '../atoms/ModalImageIndicator';
import ImageButton from '../atoms/ImageButton';
import PostButton from '../atoms/PostButton';

import { uploadImage } from '../../api/images';
import { getImageSourceForImageViewer, getLocalImageSource } from '../../utils/files';
import { foregroundColor } from '../commonColors';
import { chatStyles } from '../../utils/styles';
import OBActionSheet from '../organism/ActionSheet';

import {I18n} from '../../langs/I18n';

const MAJOR_PADDING = chatStyles.avatarImage.marginLeft;
const IMAGE_ITEM_WIDTH = 100;
const BUTTON_WRAPPER_PADDING = Platform.OS === 'ios' ? 15 : 10;

const styles = StyleSheet.create({
  wrapper: (hasImage) => ({
    height: hasImage ? IMAGE_ITEM_WIDTH + MAJOR_PADDING + 48 : MAJOR_PADDING + 48,
    paddingBottom: 12,
  }),
  itemContainer: {
    width: IMAGE_ITEM_WIDTH,
    height: IMAGE_ITEM_WIDTH,
  },
  contentContainer: {
    padding: MAJOR_PADDING / 2,
    backgroundColor: foregroundColor,
  },
  buttonsWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 15,
    paddingTop: BUTTON_WRAPPER_PADDING,
  },
});

class NewsFeedFooter extends PureComponent {
  constructor(props) {
    super(props);
    const { images } = props;
    this.state = {
      images: (images || []).map(image => ({ ...image })),
      showImageModal: false,
      initialIndex: 0,
    };
  }

  onCloseImageModal = () => {
    this.setState({ showImageModal: false });
  }

  getImageUrls = () => {
    const { images } = this.state;
    const urls = images.map(img => getImageSourceForImageViewer(get(img, 'hashes.medium'), true));
    return urls;
  }

  setImage = () => {
    this.actionSheet.show();
  }

  setTooltipRef = (ref) => {
    this.tooltip = ref;
  }

  setActionSheet = (ref) => {
    this.actionSheet = ref;
  }

  handleActionSheet = (index) => {
    if (index === 0) {
      this.useCamera();
    } else if (index === 1) {
      this.cropImage();
    }
  }

  useCamera = () => {
    ImagePicker.openCamera({
      compressImageQuality: 1,
      includeBase64: true,
      cropping: false,
      enableRotationGesture: true,
    }).then((image) => {
      this.uploadImage(image.data);
    }).catch(() => {});
  }

  cropImage = () => {
    ImagePicker.openPicker({
      width: 1000,
      height: 1000,
      compressImageQuality: 1,
      includeBase64: true,
      cropping: false,
    }).then((image) => {
      this.uploadImage(image.data);
    }).catch(() => {});
  }

  uploadImage = (imageData) => {
    const { images } = this.state;
    const { username, password } = this.props;
    const lastIndex = images.length;
    this.addImage('loading', images.length);
    uploadImage(username, password, {
      filename: 'fileImage',
      image: imageData,
    }).then((imageHashes) => {
      this.addImage(imageHashes[0], lastIndex);
    });
  }

  showImageModal = idx => () => {
    this.setState({ initialIndex: idx, showImageModal: true });
  }

  askRemoveImage = idx => () => {
    Alert.alert(
      'Delete photo?',
      'You can\'t undo this action',
      [
        { text: 'Cancel' },
        { text: 'Delete', onPress: this.removeImage(idx) },
      ],
    );
  }

  removeImage = idx => () => {
    const images = [...this.state.images];
    images.splice(idx, 1);
    this.setState({ images }, () => {
      this.props.onChange(images);
    });
    if (images.length === 0) {
      this.setState({ showImageModal: false });
    }
  }

  addImage(img, idx) {
    const images = [...this.state.images];
    images[idx] = img;
    this.setState({ images }, () => {
      this.props.onChange(images);
    });
  }

  imageKeyExtractor = (index) => `image_item_${index}`

  renderItem = ({ item, index }) => (
    <ImagePick
      style={styles.itemContainer}
      img={getLocalImageSource(get(item, 'hashes.tiny'))}
      onPress={this.showImageModal(index)}
      onRemove={this.askRemoveImage(index - 1)}
      key={`image_${index}`}
      loading={item === 'loading'}
    />
  );

  renderImageViewerHeader = (idx) => {
    const { images } = this.state;
    return (
      <React.Fragment>
        <ModalBackButton onPress={this.onCloseImageModal} />
        <ModalImageIndicator pos={idx} size={images.length} />
        <ModalTrashButton onPress={this.askRemoveImage(idx)} />
      </React.Fragment>
    );
  }

  renderIndicator = () => false

  renderImageList = () => {
    const { images } = this.state;
    if (images.length > 0) {
      return (
        <FlatList
          horizontal
          contentContainerStyle={styles.contentContainer}
          bounces={false}
          data={images}
          keyExtractor={this.imageKeyExtractor}
          renderItem={this.renderItem}
        />
      )
    }
    return false;
  }

  render() {
    const {
      images, initialIndex, showImageModal,
    } = this.state;
    const { buttonDisabled, onPost, loading } = this.props;
    const imageUrls = this.getImageUrls();
    return (
      <View style={styles.wrapper(images.length > 0)}>
        {this.renderImageList()}
        <View style={styles.buttonsWrapper}>
          <ImageButton onPress={this.setImage} />
          <PostButton loading={loading} disabled={buttonDisabled} onPress={onPost} />
        </View>
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleActionSheet}
          options={[I18n.t('components.templates.NewsFeedFooter.take_photo'), I18n.t('components.templates.NewsFeedFooter.choose_from_gallery'), I18n.t('components.templates.NewsFeedFooter.cancel')]}
          cancelButtonIndex={2}
        />
        <OBDarkModal
          onRequestClose={this.onCloseImageModal}
          visible={showImageModal}
          darkContent
        >
          <ImageViewer
            imageUrls={imageUrls}
            index={initialIndex}
            enableSwipeDown
            enablePreload
            onCancel={this.onCloseImageModal}
            loadingRender={this.renderLoadingImageViewer}
            renderHeader={this.renderImageViewerHeader}
            renderIndicator={this.renderIndicator}
          />
        </OBDarkModal>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  stage: state.createListing.stage,
});

export default connect(mapStateToProps)(NewsFeedFooter);
