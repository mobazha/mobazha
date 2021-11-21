/* eslint-disable no-return-assign */
import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Platform } from 'react-native';
import { withNavigation } from 'react-navigation';
import he from 'he';

import { setBasicInfo } from '../../reducers/createListing';
import { convertorsMap } from '../../selectors/currency';

import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';
import CheckBox from '../atoms/CheckBox';
import RadioModalFilter from '../molecules/RadioModalFilter';
import productConditionsEn from '../../config/productCondition.json';
import productConditionsZh from '../../config/zh/productCondition.json';
import prodTypesEn from '../../config/productTypes.json';
import prodTypesZh from '../../config/zh/productTypes.json';
import iosProdTypesEn from '../../config/iosProductTypes.json';
import iosProdTypesZh from '../../config/zh/iosProductTypes.json';
import { eventTracker } from '../../utils/EventTracker';
import CategorySelector from './CategorySelector';
import { getFixedCurrency } from '../../utils/currency';

import {I18n} from '../../langs/I18n';

let prodTypes = prodTypesEn
let productConditions = productConditionsEn
if (I18n.locale.startsWith('zh')) {
  prodTypes = prodTypesZh
  productConditions = productConditionsZh
}


let iosProdTypes = iosProdTypesEn
if (I18n.locale.startsWith('zh')) {
  iosProdTypes = iosProdTypesZh
}

const showingProdTypes = Platform.OS === 'ios' ?
  iosProdTypes.slice(1, iosProdTypes.length) : prodTypes.slice(1, prodTypes.length);

class ItemDetail extends PureComponent {
  constructor(props) {
    super(props);
    const {
      title, description, price, nsfw, categories, localDecimalPointsIfCrypto,
    } = props;
    this.state = {
      title,
      description: he.decode(description),
      price: price === '0' ? undefined : getFixedCurrency(parseFloat(price) || 0.0, localDecimalPointsIfCrypto || 2),
      nsfw,
      categories,
    };
  }

  onChangeType = (type) => {
    eventTracker.trackEvent('CreateListing-ChangeType', { type });
    this.props.setBasicInfo({ type });
  };

  onChangeTitle = (title) => {
    this.setState({ title }, () => { this.props.setBasicInfo(this.state); });
  };

  onChangePrice = (formated, price) => {
    this.setState({ price }, () => { this.props.setBasicInfo(this.state); });
  };

  onChangeCondition = (condition) => {
    eventTracker.trackEvent('CreateListing-ChangeCondition', { condition });
    this.props.setBasicInfo({ condition });
  };

  onChangeDescription = (description) => {
    this.setState({ description }, () => { this.props.setBasicInfo(this.state); });
  };

  onChangeNsfw = () => {
    const { nsfw } = this.state;
    this.setState({ nsfw: !nsfw }, () => { this.props.setBasicInfo(this.state); });
  };

  handleDescriptionFocus = () => {
    const { onFocusItem } = this.props;
    if (onFocusItem) {
      onFocusItem(this.descriptionInput);
    }
  }

  handleChangeCategory = (category, subCategory) => {
    const { setBasicInfo } = this.props;
    if (subCategory) {
      setBasicInfo({ categories: [category, subCategory] });
    } else {
      setBasicInfo({ categories: [] });
    }
  }

  render() {
    const {
      type, condition, localCurrency, localSymbol, productType, categories, localMask,
    } = this.props;
    const {
      title, description, price, nsfw,
    } = this.state;
    return (
      <InputGroup title={I18n.t('components.organism.ItemDetail.listing')}>
        <RadioModalFilter
          title={I18n.t('components.organism.ItemDetail.type')}
          required
          secondary
          selected={type}
          options={showingProdTypes}
          hasBorder
          onChange={this.onChangeType}
        />
        <TextInput
          title={I18n.t('components.organism.ItemDetail.title')}
          required
          placeholder={I18n.t('components.organism.ItemDetail.ask_selling')}
          value={title}
          onChangeText={this.onChangeTitle}
        />
        <TextInput
          title={I18n.t('components.organism.ItemDetail.price')}
          required
          value={price}
          onChangeText={this.onChangePrice}
          unit={localCurrency}
          mask={localMask}
          placeholder={`${localSymbol}0`}
          keyboardType="decimal-pad"
        />
        <CategorySelector onChangeCategory={this.handleChangeCategory} categories={categories} />
        {(productType.value === 'physical_good' || productType === 'physical_good') && (
          <RadioModalFilter
            title={I18n.t('components.organism.ItemDetail.condition')}
            secondary
            selected={condition}
            options={productConditions.slice(1, productConditions.length)}
            hasBorder
            onChange={this.onChangeCondition}
          />
        )}
        <TextInput
          ref={r => (this.descriptionInput = r)}
          title={I18n.t('components.organism.ItemDetail.description')}
          multiline
          noTitle
          placeholder={I18n.t('components.organism.ItemDetail.description_hint')}
          value={description}
          onChangeText={this.onChangeDescription}
          onFocus={this.handleDescriptionFocus}
        />
        <CheckBox
          checked={nsfw}
          title={I18n.t('components.organism.ItemDetail.mature_hint')}
          onPress={this.onChangeNsfw}
        />
      </InputGroup>
    );
  }
}

const mapStateToProps = state => ({
  title: state.createListing.title,
  description: state.createListing.description,
  price: state.createListing.price,
  type: state.createListing.type,
  condition: state.createListing.condition,
  categories: state.createListing.categories,
  nsfw: state.createListing.nsfw,
  localCurrency: state.settings.localCurrency,
  productType: state.createListing.type,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  setBasicInfo,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(ItemDetail));
