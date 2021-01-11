import React, { PureComponent } from 'react';
import { View, Text } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';

import InputGroup from '../atoms/InputGroup';
import RadioGroup from '../atoms/RadioGroup';
import { primaryTextColor, formLabelColor, brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  text: {
    color: primaryTextColor,
    fontSize: 15,
    marginBottom: 2,
  },
  selectedText: {
    color: brandColor,
    fontWeight: 'bold',
  },
  description: {
    color: formLabelColor,
    fontSize: 14,
    marginTop: 10,
  },
};

export default class Tabs extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      combo: props.value,
    };
  }

  onChange(idx, op) {
    const { combo } = this.state;
    const comboForOption = [...combo];
    comboForOption[idx] = op;
    this.setState(
      {
        combo: comboForOption,
      },
      () => {
        this.props.onChange(comboForOption);
      },
    );
  }

  renderItem = selected => (val, index) =>
    <Text style={[styles.text, selected === index && styles.selectedText]}>{decode(val.name)}</Text>

  renderOption(option, idx) {
    const { name, description, variants } = option;
    const { combo } = this.state;
    const comboForOption = [...combo];
    const options = variants.map((val, oid) => {
      comboForOption[idx] = oid;
      return {
        name: val.name,
        disabled: false,
      };
    });
    return (
      <InputGroup title={name} key={`option_${name}`}>
        {!_.isEmpty(description) && (
          <Text style={styles.description} numberOfLines={1}>
            {decode(description)}
          </Text>
        )}
        <RadioGroup
          selected={combo[idx]}
          options={options}
          onChange={op => this.onChange(idx, op)}
          renderItem={this.renderItem(combo[idx])}
        />
      </InputGroup>
    );
  }

  render() {
    const { options } = this.props;
    return <View>{options.map((val, idx) => this.renderOption(val, idx))}</View>;
  }
}
