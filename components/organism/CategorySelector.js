import React, { PureComponent } from 'react';
import { View, Text, Dimensions } from 'react-native';
import { withNavigation } from 'react-navigation';
import decode from 'unescape';

import { formLabelColor, primaryTextColor } from '../commonColors';
import OptionGroup from '../atoms/OptionGroup';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('screen');

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: 60,
  },
  title: {
    width: 150,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  value: {
    flex: 1,
    flexDirection: 'row',
  },
  categoryLabel: {
    maxWidth: (SCREEN_WIDTH - 32 - 150 - 18) / 2,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    paddingVertical: 20,
    paddingRight: 4,
  },
  subCategoryLabel: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    paddingVertical: 20,
    paddingRight: 4,
  },
  triggerLabel: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
    paddingVertical: 20,
    paddingRight: 4,
  },
  arrow: {
    fontSize: 15,
    color: primaryTextColor,
    paddingVertical: 20,
    paddingRight: 4,
  },
};

class CategorySelector extends PureComponent {
  handleGoToCategories = () => {
    const { navigation, onChangeCategory } = this.props;
    navigation.navigate('Categories', { onChangeCategory });
  };

  handleClear = () => {
    this.props.onChangeCategory();
  }

  render() {
    const [category, subCategory] = this.props.categories || [];
    return (
      <OptionGroup
        onPress={this.handleGoToCategories}
        fullWidth
        smallPadding
        noArrow={category}
        onClear={category && this.handleClear}
      >
        <View style={styles.wrapper}>
          <Text style={styles.title} numberOfLines={1}>
          {I18n.t('components.organism.CategorySelector.category')}
          </Text>
          {category ? (
            <View style={styles.value}>
              <Text style={styles.categoryLabel} numberOfLines={1}>{decode(category)}</Text>
              <Text style={styles.arrow}>{'>'}</Text>
              <Text style={styles.subCategoryLabel} numberOfLines={1}>{decode(subCategory)}</Text>
            </View>
          ) : (
            <Text style={styles.triggerLabel} numberOfLines={1}>{I18n.t('components.organism.CategorySelector.select_category')}</Text>
          )}
        </View>
      </OptionGroup>
    );
  }
}

export default withNavigation(CategorySelector);
