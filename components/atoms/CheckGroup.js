import React, { PureComponent } from 'react';
import { FlatList, View, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  radioWrapper: {
    flexDirection: 'row',
    paddingHorizontal: 16,
    alignItems: 'center',
  },
};

export default class CheckGroup extends PureComponent {
  isSelected = (val) => {
    const { selected, compare } = this.props;
    return selected.find(o => compare(val, o));
  }

  renderItem = ({ item }) => {
    const { onChange, renderItem } = this.props;
    return (
      <TouchableWithoutFeedback
        onPress={() => onChange(item, !this.isSelected(item))}
        key={`cg_${item.value}`}
      >
        <View style={styles.radioWrapper}>
          {renderItem(item)}
          <View style={{ flex: 1 }} />
          {this.isSelected(item) && <Ionicons name="md-checkmark" size={22} color={brandColor} />}
        </View>
      </TouchableWithoutFeedback>
    );
  }

  render() {
    const { options } = this.props;
    return (
      <FlatList
        data={options}
        renderItem={this.renderItem}
      />
    );
  }
}
