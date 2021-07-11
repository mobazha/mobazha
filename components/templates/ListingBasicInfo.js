/* eslint-disable react/sort-comp */
import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { findNodeHandle, View, Platform, Keyboard } from 'react-native';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';

import InputGroup from '../atoms/InputGroup';
import FormLabelText from '../atoms/FormLabelText';
import ItemDetail from '../organism/ItemDetail';
import ShippingOptions from '../organism/ShippingOptions';
import TagEditor from '../organism/TagEditor';
import ImageSelector from '../organism/ImageSelector';

import { backgroundColor } from '../commonColors';
import { setImages, setBasicInfo } from '../../reducers/createListing';
import OptionGroup from '../atoms/OptionGroup';
import MoreButton from '../atoms/MoreButton';
import { eventTracker } from '../../utils/EventTracker';
import { screenWrapper } from '../../utils/styles';
import EditListingFooter from '../atoms/EditListingFooter';
import ListingHeader from '../molecules/ListingHeader';
import { StatusBarSpacer } from '../../status-bar';

import {I18n} from '../../langs/I18n';
class ListingBasicInfo extends PureComponent {
  state = { keyboardVisible: false }

  selectorRef = React.createRef();

  componentDidMount() {
    this.keyboardDidShowSub = Keyboard.addListener('keyboardDidShow', this.keyboardDidShow);
    this.keyboardWillHideSub = Keyboard.addListener('keyboardDidHide', this.keyboardWillHide);
  }

  keyboardDidShow = () => {
    this.setState({ keyboardVisible: true });
  };

  keyboardWillHide = () => {
    this.setState({ keyboardVisible: false });
  };

  onChangeImages = (images) => {
    eventTracker.trackEvent('CreateListing-AttachedPhoto');
    this.props.setImages(images);
  };

  getScrollRef = (ref) => {
    this.scroll = ref;
  };

  toShippingOptions = (option) => {
    this.props.toOptions(option);
  };

  toAdvancedOptions = () => {
    this.props.toOptions('ListingAdvancedOptions');
  };

  toTagEditor = () => {
    this.props.toOptions('TagEditor');
  };

  focusInput = (evt) => {
    this.scroll.props.scrollToFocusedInput(findNodeHandle(evt.target));
  };

  handleAddImage = () => {
    this.selectorRef.current.wrappedInstance.mainActionSheet.show();
  }

  handlePressMore = () => {
    this.selectorRef.current.wrappedInstance.moreActionSheet.show();
  }

  render() {
    const {
      images, productType, tags, onBack, onSave,
    } = this.props;

    const { keyboardVisible } = this.state;

    const wrapperStyle = {
      backgroundColor,
      flex: 1,
    };
    return (
      <View style={screenWrapper.wrapper}>
        <ListingHeader
          editing
          showActions={images.length > 0}
          onBack={onBack}
          action={this.handleAddImage}
          onMore={this.handlePressMore}
        />
        <StatusBarSpacer />
        <KeyboardAwareScrollView
          style={wrapperStyle}
          innerRef={this.getScrollRef}
          keyboardShouldPersistTaps="always"
        >
          <ImageSelector ref={this.selectorRef} images={images} onChange={this.onChangeImages} />
          <ItemDetail onFocusItem={this.handleInputOnFocus} />
          {(productType.value === 'physical_good' || productType === 'physical_good') && (
            <ShippingOptions onPress={this.toShippingOptions} />
          )}
          <TagEditor count={tags.length} onPress={this.toTagEditor} />

          <InputGroup title={I18n.t('components.templates.ListingBasicInfo.advanced')} noBorder onPress={this.toAdvancedOptions}>
            <OptionGroup noBorder noArrow>
              <React.Fragment>
                <FormLabelText text={I18n.t('components.templates.ListingBasicInfo.advanced_description')} />
                <MoreButton onPress={this.toAdvancedOptions} />
              </React.Fragment>
            </OptionGroup>
          </InputGroup>
          {keyboardVisible && <EditListingFooter onSave={onSave} />}
        </KeyboardAwareScrollView>
        {!keyboardVisible && <EditListingFooter onSave={onSave} />}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  images: state.createListing.images,
  categories: state.createListing.categories,
  productType: state.createListing.type,
  tags: state.createListing.tags,
  navigation: state.nav,
});

const mapDispatchToProps = {
  setImages,
  setBasicInfo,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ListingBasicInfo);
