import React, { PureComponent } from 'react';
import { View, Text } from 'react-native';

import { primaryTextColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    paddingVertical: 12,
    paddingLeft: 20,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderColor,
  },
  textStyle: {
    flex: 1,
    color: primaryTextColor,
    fontSize: 15,
    paddingRight: 15,
  },
};

export default class SearchFilterHeader extends PureComponent {
  render() {
    const {
      total,
    } = this.props;
    return (
      <View style={styles.wrapper}>
        {total ? (<Text style={styles.textStyle} numberOfLines={1}>{I18n.t('components.organism.SearchFilterHeader.results', {total: total})}</Text>)
          : (<View style={{ flex: 1 }} />)}
      </View>
    );
  }
}
