/* eslint-disable react/sort-comp */
import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Dimensions, TouchableWithoutFeedback, Text, View, Alert, Image } from 'react-native';
import ImagePicker from 'react-native-image-crop-picker';
import ImageViewer from 'react-native-image-zoom-viewer';
import { get } from 'lodash';
import RNFS from 'react-native-fs';
import Carousel from 'react-native-snap-carousel';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { OBDarkModal } from '../templates/OBModal';
import ModalBackButton from '../atoms/ModalBackButton';
import ModalTrashButton from '../atoms/ModalTrashButton';
import ModalFavButton from '../atoms/ModalFavButton';
import ModalTooltip from '../atoms/ModalTooltip';
import ModalImageIndicator from '../atoms/ModalImageIndicator';
import { uploadImage } from '../../api/images';
import { getImageSourceForImageViewer, getImageSourceWithDefault } from '../../utils/files';
import { backgroundColor } from '../commonColors';
import OBActionSheet from './ActionSheet';

import {I18n} from '../../langs/I18n';

const DEVICE_WIDTH = Dimensions.get('window').width;

const styles = {
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(108, 108, 108, 0.5)',
    borderRadius: 16,
    height: 32,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    bottom: 16,
    right: 16,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  empty: {
    width: DEVICE_WIDTH,
    height: DEVICE_WIDTH,
    backgroundColor: '#c0c0c0',
    alignItems: 'center',
    justifyContent: 'center',
  },
  image: {
    width: DEVICE_WIDTH,
    height: DEVICE_WIDTH,
  },
};

class ImageSelector extends PureComponent {
  carouselRef = React.createRef();

  constructor(props) {
    super(props);
    this.state = { images: [...this.props.images], initialIndex: -1, activeSlide: 0 };
  }

  onCloseImageModal = () => {
    this.setState({ initialIndex: -1 });
  }

  getImageUrls = () => {
    const { images } = this.state;
    const urls = images.map(img => getImageSourceForImageViewer(get(img, 'hashes.medium'), true));
    return urls;
  }

  setMainActionSheet = (ref) => {
    this.mainActionSheet = ref;
  }

  setMoreActionSheet = (ref) => {
    this.moreActionSheet = ref;
  }

  setMainTooltipRef = (ref) => {
    this.mainTooltip = ref;
  }

  setModalTooltipRef = (ref) => {
    this.modalTooltip = ref;
  }

  setImage = idx => () => {
    const imageUrls = this.getImageUrls();
    const imageUrl = imageUrls[idx];
    if (imageUrl.url) {
      this.setState({ initialIndex: idx });
    }
  }

  setAsFavorite = idx => () => {
    const { images: origImages, initialIndex } = this.state;
    const images = [...origImages];
    images.unshift(images.splice(idx, 1)[0]);

    this.setState({ images, initialIndex: initialIndex > -1 ? 0 : -1 }, () => {
      this.carouselRef.current.snapToItem(0);
    });

    this.props.onChange(images);

    if (initialIndex > -1) {
      this.modalTooltip.show();
    } else {
      this.mainTooltip.show();
    }
  }

  handleMainActionSheet = (index) => {
    if (index === 0) {
      this.useCamera();
    } else if (index === 1) {
      this.cropImage();
    }
  }

  handleMoreActionSheet = (index) => {
    const { activeSlide } = this.state;
    if (index === 0) {
      this.setAsFavorite(activeSlide)();
    } else if (index === 1) {
      this.askRemoveImage(activeSlide)();
    }
  }

  handleSnapToItem = (index) => {
    this.setState({ activeSlide: index });
  }

  useCamera = () => {
    ImagePicker.openCamera({
      compressImageQuality: 1,
      includeBase64: true,
      cropping: false,
      enableRotationGesture: true,
    })
      .then(this.uploadImage)
      .catch(() => {});
  }

  cropImage = () => {
    ImagePicker.openPicker({
      width: 1000,
      height: 1000,
      compressImageQuality: 1,
      includeBase64: true,
      cropping: false,
    })
      .then(this.uploadImage)
      .catch(() => {});
  }

  uploadImage = async (image) => {
    const { username, password } = this.props;
    const { images } = this.state;

    const curPos = images.length;
    this.addImage('loading', curPos);

    const imageData = await RNFS.readFile(image.path, 'base64');
    uploadImage(username, password, {
      filename: 'fileImage', image: imageData,
    }).then((imageHashes) => {
      this.addImage(imageHashes[0], curPos);
    });
  }

  askRemoveImage = idx => () => {
    Alert.alert(
      I18n.t('components.organism.ImageSelector.ask_delete'),
      I18n.t('components.organism.ImageSelector.cannot_undo'),
      [
        { text: I18n.t('components.organism.ImageSelector.cancel') },
        { text: I18n.t('components.organism.ImageSelector.delete'), onPress: this.removeImage(idx) },
      ],
    );
  }

  removeImage = idx => () => {
    const { images, activeSlide, initialIndex } = this.state;
    const newImages = [...images];
    newImages.splice(idx, 1);

    const changeImages = () => this.props.onChange(newImages);

    if (newImages.length === 0) {
      this.setState({ initialIndex: -1, images: newImages }, changeImages);
    } else {
      const newActiveSlide = Math.max(activeSlide - 1, 0);
      const newInitialIndex = initialIndex === -1 ? -1 : Math.max(initialIndex - 1, 0);
      this.setState({
        images: newImages, activeSlide: newActiveSlide, initialIndex: newInitialIndex,
      }, () => {
        this.carouselRef.current.snapToItem(newActiveSlide);
        changeImages();
      });
    }
  }

  addImage = (img, idx) => {
    const images = [...this.state.images];
    images[idx] = img;
    this.setState({ images }, () => {
      this.carouselRef.current.snapToItem(idx);
      this.props.onChange(images);
    });
  }

  renderItem = ({ item, index }) => {
    return (
      <TouchableWithoutFeedback activeOpacity={1} key={index} onPress={this.setImage(index)}>
        <View>
          <Image
            style={styles.image}
            source={getImageSourceWithDefault(get(item, 'hashes.medium', ''))}
            resizeMode="cover"
          />
        </View>
      </TouchableWithoutFeedback>
    );
  };

  renderEmptyItem = () => (
    <TouchableWithoutFeedback onPress={() => this.mainActionSheet.show()}>
      <View style={styles.empty}>
        <Ionicons name="md-camera" size={32} color={backgroundColor} />
      </View>
    </TouchableWithoutFeedback>
  );

  renderImageViewerHeader = (idx) => {
    const { images } = this.state;
    return (
      <React.Fragment>
        <ModalBackButton onPress={this.onCloseImageModal} />
        <ModalImageIndicator pos={idx} size={images.length} />
        <ModalFavButton selected={idx === 0} onPress={this.setAsFavorite(idx)} />
        <ModalTrashButton onPress={this.askRemoveImage(idx)} />
        <ModalTooltip ref={this.setModalTooltipRef} text={I18n.t('components.organism.ImageSelector.set_primary')} />
      </React.Fragment>
    );
  }

  renderIndicator = () => null

  render() {
    const { images, initialIndex, activeSlide } = this.state;
    const imageUrls = this.getImageUrls();
    return (
      <View>
        {images.length > 0 ? (
          <Carousel
            ref={this.carouselRef}
            data={images}
            renderItem={this.renderItem}
            sliderWidth={DEVICE_WIDTH}
            itemWidth={DEVICE_WIDTH}
            itemHeight={DEVICE_WIDTH}
            sliderHeight={DEVICE_WIDTH}
            onSnapToItem={this.handleSnapToItem}
            enableSnap
            inactiveSlideOpacity={1}
          />
        ) : (
          this.renderEmptyItem()
        )}
        {images.length > 0 && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{activeSlide + 1}/{images.length}</Text>
          </View>
        )}
        <OBDarkModal
          onRequestClose={this.onCloseImageModal}
          visible={initialIndex > -1}
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
        <ModalTooltip ref={this.setMainTooltipRef} text={I18n.t('components.organism.ImageSelector.set_primary')} />
        <OBActionSheet
          ref={this.setMainActionSheet}
          onPress={this.handleMainActionSheet}
          options={[I18n.t('components.organism.ImageSelector.take_photo'), I18n.t('components.organism.ImageSelector.choose_from_gallery'), I18n.t('components.organism.ImageSelector.cancel')]}
          cancelButtonIndex={2}
        />
        <OBActionSheet
          ref={this.setMoreActionSheet}
          onPress={this.handleMoreActionSheet}
          options={[I18n.t('components.organism.ImageSelector.set_primary'), I18n.t('components.organism.ImageSelector.delte_photo'), I18n.t('components.organism.ImageSelector.cancel')]}
          cancelButtonIndex={2}
          destructiveButtonIndex={1}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  images: state.createListing.images,
  stage: state.createListing.stage,
});

export default connect(mapStateToProps, null, null, { withRef: true })(ImageSelector);
