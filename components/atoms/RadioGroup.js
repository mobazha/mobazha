import React, { PureComponent } from 'react';
import { View, TouchableWithoutFeedback, FlatList } from 'react-native';
import MaterialCommunityIcons from 'react-native-vector-icons/MaterialCommunityIcons';
import { borderColor, greenColor, staticLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingVertical: 10,
  },
  itemWrapper: {},
  radioWrapper: {
    flexDirection: 'row',
    paddingVertical: 10,
    alignItems: 'center',
  },
  alignTop: {
    alignItems: 'flex-start',
  },
  checkmark: {
    marginRight: 10,
  },
  disabled: {},
  border: {
    borderBottomWidth: 1,
    borderColor,
  },
};

export default class RadioGroup extends PureComponent {
  onSelect = (item, index) => () => {
    const { onChange } = this.props;
    if (!item.disabled) {
      onChange(index);
    }
  }

  keyExtractor = (item, index) => `radioOption_${index}`;

  renderItem = ({ item, index }) => {
    const {
      selected, options, renderItem, showSeparator, alignTop,
    } = this.props;
    return (
      <View style={styles.itemWrapper}>
        <TouchableWithoutFeedback
          onPress={this.onSelect(item, index)}
          disabled={item.disabled}
        >
          <View
            style={[
              styles.radioWrapper,
              item.disabled && styles.disabled,
              index !== options.length - 1 && showSeparator && styles.border,
              alignTop && styles.alignTop,
            ]}
          >
            {index === selected ? (
              <MaterialCommunityIcons name="radiobox-marked" color={greenColor} size={20} style={styles.checkmark} />
            ) : (
              <MaterialCommunityIcons name="radiobox-blank" color={item.disabled ? '#979797' : staticLabelColor} size={20} style={styles.checkmark} />
            )}
            {renderItem(item, index)}
          </View>
        </TouchableWithoutFeedback>
      </View>
    );
  }
  render() {
    const { options, selected } = this.props;
    return (
      <View style={styles.wrapper}>
        <FlatList
          data={options}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderItem}
          extraData={selected}
        />
      </View>
    );
  }
}
