import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { ScrollView, View, Animated, Text, Platform } from 'react-native';
import { set } from 'lodash';

import { updateFilter } from '../reducers/search';

import RadioFilter from '../components/molecules/RadioFilter';
import RadioModalFilter from '../components/molecules/RadioModalFilter';
import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import LinkText from '../components/atoms/LinkText';
import SwitchInput from '../components/atoms/SwitchInput';
import ResetFilter from '../components/atoms/ResetFilter';
import Section from '../components/molecules/Section';

import { screenWrapper } from '../utils/styles';
import { ratingOptions } from '../utils/ratings';

import { ACCEPTED_COINS } from '../utils/coins';
import prodTypesEn from '../config/productTypes.json';
import prodTypesZh from '../config/zh/productTypes.json';
import countries from '../config/countries.json';
import sortOptionsEn from '../config/sort.json';
import sortOptionsZh from '../config/zh/sort.json';
import productConditionOptionsEn from '../config/conditionFilter.json';
import productConditionOptionsZh from '../config/zh/conditionFilter.json';

import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

let prodTypes = prodTypesEn
let sortOptions = sortOptionsEn
let productConditionOptions = productConditionOptionsEn
if (I18n.locale.startsWith('zh')) {
  prodTypes = prodTypesZh
  sortOptions = sortOptionsZh
  productConditionOptions = productConditionOptionsZh
}

const shippingCountries = [...countries];

const showingProdTypes = Platform.OS === 'ios' ? prodTypes.filter(item => item.value !== 'digital_good') : prodTypes;

const optionStyle = {
  paddingHorizontal: 14,
};

const toastStyle = {
  container: {
    position: 'absolute',
    bottom: 80,
    justifyContent: 'center',
    alignSelf: 'center',
  },
  wrapper: {
    width: 'auto',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    paddingHorizontal: 25,
    paddingVertical: 6,
    justifyContent: 'center',
  },
  text: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class SearchFilter extends PureComponent {
  constructor(props) {
    super(props);
    const { filter } = props;
    this.state = { ...filter, showFilterToast: false };
  }

  UNSAFE_componentWillMount() {
    const { filter } = this.props;
    this.setState({ ...filter });
  }

  onLeft = () => {
    this.props.navigation.goBack();
  }

  onRight = () => {
    const {
      shipping, rating, type, sortBy, nsfw, conditions, acceptedCurrencies,
    } = this.state;
    eventTracker.trackEvent('Discover-FilteredSearch', {
      shipping,
      rating,
      type,
      sortBy,
      nsfw,
      conditions,
      acceptedCurrencies,
    });
    this.props.updateFilter({
      shipping,
      rating,
      type,
      sortBy,
      nsfw,
      conditions,
      acceptedCurrencies,
    });
    this.props.navigation.goBack();
  }

  onChange = field => (val) => {
    const updateObject = {};
    set(updateObject, field, val);
    this.setState(updateObject);
  }

  aniVal = new Animated.Value(0);

  resetFilters = () => {
    this.setState({
      shipping: 'any',
      rating: 0,
      type: 'any',
      sortBy: 'relevance',
      nsfw: false,
      conditions: 'any',
      acceptedCurrencies: 'any',
      showFilterToast: true,
    }, () => {
      Animated.timing(this.aniVal, {
        toValue: 1,
        duration: 1000,
      }).start(() => {
        Animated.timing(this.aniVal, {
          toValue: 0,
          duration: 1000,
          delay: 2000,
        }).start(() => {
          this.setState({
            showFilterToast: false,
          });
        });
      });
    });
  }

  render() {
    const {
      shipping, rating, nsfw, acceptedCurrencies, sortBy, type, conditions,
      showFilterToast,
    } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.onLeft}
          title={I18n.t('screens.searchFilter.filter')}
          right={<LinkText text="Done" />}
          onRight={this.onRight}
          noBorder
        />
        <ScrollView>
          <RadioFilter
            title={I18n.t('screens.searchFilter.sortBy')}
            selected={sortBy}
            options={sortOptions}
            onChange={this.onChange('sortBy')}
            hasBorder
          />
          <RadioModalFilter
            title={I18n.t('screens.searchFilter.accepts')}
            selected={acceptedCurrencies}
            options={ACCEPTED_COINS}
            onChange={this.onChange('acceptedCurrencies')}
            hasBorder
          />
          <RadioModalFilter
            title={I18n.t('screens.searchFilter.ships_to')}
            options={shippingCountries}
            selected={shipping}
            onChange={this.onChange('shipping')}
            hasBorder
          />
          <RadioFilter
            title={I18n.t('screens.searchFilter.rating')}
            selected={rating}
            options={ratingOptions}
            onChange={this.onChange('rating')}
            hasBorder
          />
          <RadioFilter
            title={I18n.t('screens.searchFilter.listing_type')}
            selected={type}
            options={showingProdTypes}
            onChange={this.onChange('type')}
            hasBorder
          />
          {type === 'physical_good' && (
            <RadioFilter
              title={I18n.t('screens.searchFilter.item_condition')}
              selected={conditions}
              options={productConditionOptions}
              onChange={this.onChange('conditions')}
              hasBorder
            />
          )}
          <Section title= {I18n.t('screens.searchFilter.adult_content')} bodyStyle={optionStyle}>
            <SwitchInput
              secondary
              noBorder
              useNative
              title= {I18n.t('screens.searchFilter.adult_content2')} 
              value={nsfw}
              onChange={this.onChange('nsfw')}
            />
          </Section>
        </ScrollView>
        { showFilterToast && (
          <Animated.View
            style={[
              toastStyle.container,
              {
                opacity: this.aniVal,
              },
            ]}
          >
            <View style={toastStyle.wrapper}>
              <Text style={toastStyle.text}>
              {I18n.t('screens.searchFilter.filters_reset')} 
              </Text>
            </View>
          </Animated.View>
        )}
        <ResetFilter onPress={this.resetFilters} />
      </View>
    );
  }
}

const mapStateToProps = state => ({ filter: state.search.filter });

const mapDispatchToProps = { updateFilter };

export default connect(mapStateToProps, mapDispatchToProps)(SearchFilter);
