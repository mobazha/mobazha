import React, { PureComponent } from 'react';
import { View, FlatList, Text, TouchableWithoutFeedback } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';
import { connect } from 'react-redux';

import { screenWrapper } from '../utils/styles';
import categories from '../config/categories';

import ProductCard, { styles as cardStyles } from '../components/molecules/ProductCard';
import { getRandomSearch } from '../api/search';
import InputGroup from '../components/atoms/InputGroup';
import { foregroundColor, primaryTextColor } from '../components/commonColors';
import { renderTextHeader } from '../components/molecules/TextHeader';
import SearchHeader from '../components/organism/SearchHeader';
import ProductCardLoader from '../components/atoms/ProductCardLoader';
import { clearFilter } from '../reducers/search';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const styles = {
  showAllButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    marginVertical: 12,
  },
  showAllText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  subWrapper: {
    backgroundColor: foregroundColor,
    paddingLeft: 16,
    paddingRight: 12,
    paddingVertical: 10,
  },
  sub: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    lineHeight: 30,
  },
  subCategoriesHeader: {
    paddingLeft: 0,
  },
  loaderWrapper: {
    marginTop: 16,
  },
};

class CategoryOverview extends PureComponent {
  constructor(props) {
    super(props);

    const { navigation } = props;
    const { name } = navigation.getParam('item');
    const categoryObject = categories.find(category => category.name === name);
    this.subs = (categoryObject && categoryObject.subs) || [];
  }

  state = { results: [] }

  async componentDidMount() {
    const { name } = this.props.navigation.getParam('item');
    const response = await getRandomSearch(name, '', 0, 6, '');
    const results = _.get(response, 'results.results');
    if (results) {
      this.setState({ results });
    }
  }

  getInterpolatedResults = () => {
    const { results } = this.state;
    return Array.from(Array(6).keys()).map(index => results[index] || {});
  }

  subCategoriesHeader = renderTextHeader('Subcategories');

  handleGoBack = () => {
    this.props.navigation.goBack();
  };

  handleGoToListing = (params) => {
    const { navigation } = this.props;
    const { name } = navigation.getParam('item');
    eventTracker.trackEvent('Discover-TappedListing-CategoryOverview', {
      reference: 'CategoryOverview', category: name,
    });
    navigation.navigate('Listing', params);
  }

  handleShowAll = () => {
    const { navigation } = this.props;
    const item = navigation.getParam('item');
    eventTracker.trackEvent('Discover-Tapped-CategorySeeAll', { category: item.name });
    navigation.navigate('CategoryResult', { item });
  };

  handleGoToSub = sub => () => {
    const { navigation } = this.props;
    const item = navigation.getParam('item');
    eventTracker.trackEvent('Discover-Tapped-SubCategory', { category: item.name, subCategory: sub });
    navigation.navigate('CategoryResult', { item, sub });
  }

  handleClickHeader = () => {
    const { clearFilter, navigation } = this.props;
    clearFilter();
    navigation.navigate('CategoryResult', {
      item: navigation.getParam('item'),
      searchAutoFocus: true,
    });
  }

  keyExtractor = (val, index) => `infinite_${index}`

  renderItem = ({ item }) => {
    const { data } = item;
    if (!data) {
      return <ProductCard compact />;
    }

    return (
      <ProductCard
        slug={data.slug}
        title={decode(data.title)}
        thumbnail={_.get(data, 'thumbnail.small')}
        averageRating={data.averageRating}
        ratingCount={data.ratingCount}
        currencyCode={_.get(data, 'bigPrice.currencyCode', '')}
        amount={_.get(data, 'bigPrice.amount', 0)}
        peerID={_.get(data, 'relationships.vendor.data.peerID')}
        hash={data.hash}
        onPress={() => this.handleGoToListing(data)}
        compact
      />
    );
  }

  renderSub = ({ item }) => (
    <TouchableWithoutFeedback onPress={this.handleGoToSub(item)}>
      <View style={styles.subWrapper}>
        <Text style={styles.sub}>{item}</Text>
      </View>
    </TouchableWithoutFeedback>
  )

  render() {
    const { navigation } = this.props;
    const { name } = navigation.getParam('item');
    const SubCategoriesHeader = this.subCategoriesHeader;
    const { results } = this.state;

    return (
      <View style={screenWrapper.wrapper}>
        <SearchHeader onClick={this.handleClickHeader} navigation={navigation} onBack={this.handleGoBack} lightStatus />
        <FlatList
          ListHeaderComponent={
            <InputGroup title={name} noBorder>
              {results.length === 0 ? (
                <View style={styles.loaderWrapper}>
                  <ProductCardLoader compact />
                  <ProductCardLoader compact />
                </View>
              ) : (
                <FlatList
                  numColumns={3}
                  columnWrapperStyle={cardStyles.compactColumnWrapperStyle}
                  keyExtractor={this.keyExtractor}
                  horizontal={false}
                  data={this.getInterpolatedResults()}
                  renderItem={this.renderItem}
                />
              )}
              <TouchableWithoutFeedback onPress={this.handleShowAll}>
                <View style={styles.showAllButton}>
                  <Text style={styles.showAllText}>{I18n.t('screens.categoryOverview.see_all')}</Text>
                </View>
              </TouchableWithoutFeedback>
              <SubCategoriesHeader style={styles.subCategoriesHeader} />
            </InputGroup>
          }
          data={this.subs}
          keyExtractor={(item, index) => `sub_item_${index}`}
          renderItem={this.renderSub}
          keyboardDismissMode="interactive"
          keyboardShouldPersistTaps="handled"
        />
      </View>
    );
  }
}

const mapDispatchToProps = {
  clearFilter,
};

export default connect(
  null,
  mapDispatchToProps,
)(CategoryOverview);
