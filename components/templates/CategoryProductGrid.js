import React, { PureComponent } from 'react';
import * as _ from 'lodash';
import { View } from 'react-native';
import { withNavigation } from 'react-navigation';
import { connect } from 'react-redux';

import InputGroup from '../atoms/InputGroup';
import ProductGrid from '../organism/ProductGrid';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    marginTop: 16,
  },
  emptyWrapper: {
    height: 64,
    justifyContent: 'center',
    alignItems: 'center',
  },
  actionTitle: {
    fontWeight: 'bold',
  },
};

class CategoryOverview extends PureComponent {
  handleGoToListing = (params) => {
    this.props.navigation.navigate('Listing', params);
  }

  render() {
    const { title, productCount, onPress, listingsForCategories, name } = this.props;
    const { data: results, loading } = (listingsForCategories[name] || {});
    if (loading === false && (results ==null || results.length === 0)) {
      return null;
    }

    return (
      <InputGroup
        title={title}
        action={onPress}
        actionTitle={I18n.t('components.templates.CategoryProductGrid.see_all')}
        actionStyle={styles.actionTitle}
        noPadding
        noBorder
      >
        <View style={styles.wrapper}>
          <ProductGrid
            products={results}
            keyPrefix="category_item"
            toListingDetails={this.handleGoToListing}
            count={productCount}
          />
        </View>
      </InputGroup>
    );
  }
}

const mapStateToProps = state => ({
  listingsForCategories: state.product.listingsForCategories,
});

export default withNavigation(connect(
  mapStateToProps,
)(CategoryOverview));
