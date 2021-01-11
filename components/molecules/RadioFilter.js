import React, { PureComponent } from 'react';
import { TouchableWithoutFeedback, Text, View, FlatList } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import MaterialCommunityIcons from 'react-native-vector-icons/MaterialCommunityIcons';
import * as _ from 'lodash';

import { foregroundColor, greenColor, primaryTextColor, linkTextColor, borderColor, staticLabelColor, brandColor } from '../commonColors';

import Section from './Section';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
  },
  itemWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 14,
  },
  radioButton: {
    color: greenColor,
    marginRight: 5,
    marginTop: 3,
  },
  checkmark: {
    marginRight: 14,
    width: 20,
    marginTop: 2,
  },
  itemLabelWrapper: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 15,
  },
  selectedText: {
    fontWeight: 'bold',
    color: linkTextColor,
  },
  itemText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  border: {
    borderBottomWidth: 1,
    borderColor,
    flex: 1,
  },
};

export default class RadioFilter extends PureComponent {
  onChange = val => () => { this.props.onChange(val); }

  keyExtractor = (item, index) => `option_${index}`;

  renderLabel = (item, selected) => {
    const { valueKey, getLabel } = this.props;
    const value = _.isEmpty(valueKey) ? _.get(item, 'value') : _.get(item, valueKey);
    const label = getLabel ? getLabel(item) : _.get(item, 'label');
    if (_.isString(label)) {
      return <Text style={[styles.itemText, selected === value ? styles.selectedText : {}]}>{label}</Text>;
    }
    if (_.isArray(label)) {
      return label.map(labelItem => (labelItem));
    }
    return false;
  }

  renderItem = ({ item }) => {
    const { selected, valueKey, secondary } = this.props;
    const value = _.isEmpty(valueKey) ? _.get(item, 'value') : _.get(item, valueKey);
    return (
      <TouchableWithoutFeedback onPress={this.onChange(value)}>
        <View style={styles.itemWrapper}>
          {secondary && selected === value && (
            <MaterialCommunityIcons name="radiobox-marked" color={greenColor} size={20} style={styles.checkmark} />
          )}
          {secondary && selected !== value && (
            <MaterialCommunityIcons name="radiobox-blank" color={staticLabelColor} size={20} style={styles.checkmark} />
          )}
          <View style={styles.itemLabelWrapper}>{this.renderLabel(item, selected)}</View>
          {!secondary && selected === value && (
            <Ionicons name="md-checkmark" size={17} color={brandColor} style={styles.checkmark} />
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }

  render() {
    const {
      selected, options, title, style, hasBorder, wrapperStyle,
    } = this.props;
    return (
      <Section
        title={title}
        style={wrapperStyle}
        bodyStyle={hasBorder ? styles.border : { flex: 1 }}
      >
        <FlatList
          data={options}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderItem}
          extraData={selected}
          style={[styles.wrapper, style]}
          alwaysBounceVertical={false}
        />
      </Section>
    );
  }
}
