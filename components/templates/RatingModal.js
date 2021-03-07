import React, { PureComponent } from 'react';
import { isEmpty } from 'lodash';
import { findNodeHandle } from 'react-native';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';

import Header from '../molecules/Header';
import RatingInput from '../atoms/RatingInput';
import CheckBox from '../atoms/CheckBox';
import { primaryTextColor, linkTextColor, borderColor } from '../commonColors';
import NavCloseButton from '../atoms/NavCloseButton';
import { OBLightModal } from './OBModal';
import LinkText from '../atoms/LinkText';
import TextArea from '../atoms/TextArea';

import {I18n} from '../../langs/I18n';

const styles = {
  scrollview: {
    flex: 1,
  },
  content: {
    flexGrow: 1,
    paddingHorizontal: 17,
  },
  done: {
    fontSize: 15,
    color: linkTextColor,
    textAlign: 'right',
  },
  inputText: {
    fontWeight: 'normal',
    flex: 1,
    fontSize: 15,
    color: primaryTextColor,
    borderBottomWidth: 1,
    borderColor,
    paddingVertical: 12,
    marginTop: 12,
    height: 138,
  },
};

export default class RatingModal extends PureComponent {
  state = {
    overall: 0,
    quality: 0,
    description: 0,
    deliverySpeed: 0,
    customerService: 0,
    review: '',
    anonymous: false,
  };

  handleChange = key => (value) => {
    const dict = {};
    dict[key] = value;
    this.setState({ ...dict });
  };

  scrollToInput(reactNode) {
    this.scroll.props.scrollToFocusedInput(reactNode);
  }

  handleFocus = (event) => {
    this.scrollToInput(findNodeHandle(event.target));
  }

  render() {
    const {
      show, order, hideModal, finishReview,
    } = this.props;
    const {
      overall,
      quality,
      description,
      deliverySpeed,
      customerService,
      review,
      anonymous,
    } = this.state;
    return (
      <OBLightModal visible={show} animationType="slide">
        <Header
          modal
          left={<NavCloseButton />}
          onLeft={hideModal}
          right={<LinkText text={I18n.t('components.templates.RatingModal.done')} color={linkTextColor} />}
          onRight={() => finishReview(order.orderId, [{ slug: order.slug, ...this.state }])}
        />
        <KeyboardAwareScrollView
          style={styles.scrollview}
          contentContainerStyle={styles.content}
          innerRef={(ref) => {
            this.scroll = ref;
          }}
        >
          <RatingInput
            title={I18n.t('components.templates.RatingModal.Overall')}
            value={overall}
            onPress={this.handleChange('overall')}
          />
          <RatingInput
            title= {I18n.t('components.templates.RatingModal.Quality')}
            value={quality}
            onPress={this.handleChange('quality')}
          />
          <RatingInput
            title={I18n.t('components.templates.RatingModal.As_advertised')}
            value={description}
            onPress={this.handleChange('description')}
          />
          <RatingInput
            title= {I18n.t('components.templates.RatingModal.Delivery')}
            value={deliverySpeed}
            onPress={this.handleChange('deliverySpeed')}
          />
          <RatingInput
            title={I18n.t('components.templates.RatingModal.Service')}
            value={customerService}
            onPress={this.handleChange('customerService')}
          />
          <TextArea
            style={styles.inputText}
            noBorder
            value={review}
            onChangeText={this.handleChange('review')}
            placeholder={I18n.t('components.templates.RatingModal.Write_a_review')}
            onFocus={this.handleFocus}
          />
          <CheckBox
            checked={anonymous}
            title= {I18n.t('components.templates.RatingModal.Post_anonymously')}
            onPress={() => {
              this.setState({
                anonymous: !anonymous,
              });
            }}
          />
        </KeyboardAwareScrollView>
      </OBLightModal>
    );
  }
}
