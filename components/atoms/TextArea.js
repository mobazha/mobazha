import React, { PureComponent } from 'react';
import { View, Text, TextInput } from 'react-native';
import { isEmpty } from 'lodash';

import { primaryTextColor, formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    paddingVertical: 15,
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
  input: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    paddingVertical: 0,
    paddingHorizontal: 0,
    textAlignVertical: 'top',
    marginTop: -5,
  },
};

export default class TextArea extends PureComponent {
  constructor(props) {
    super(props);
    this.state = { value: props.value || props.defaultValue || '' };
  }

  componentDidUpdate(prevProps, prevState, valueUpdated) {
    if (valueUpdated) {
      this.setState({ value: this.props.value });
    }
  }

  onChangeText = (value) => {
    const { onChangeText } = this.props;
    this.setState({ value });
    if (onChangeText) {
      this.props.onChangeText(value);
    }
  }

  getSnapshotBeforeUpdate(preProps, prevState) {
    if (prevState.value !== this.props.value) {
      return true;
    }
    return false;
  }

  setInputRef = (ref) => { this.input = ref; }

  focus() {
    if (this.input) {
      this.input.focus();
    }
  }

  blur() {
    if (this.input) {
      this.input.blur();
    }
  }

  render() {
    const {
      style,
      title,
      onChange,
      noBorder,
      defaultValue,
      onFocus,
      placeholder,
      autoFocus,
    } = this.props;
    const { value } = this.state;
    return (
      <View style={[styles.wrapper, !noBorder && { borderBottomWidth: 1, borderColor: '#E8E8E8' }, style]}>
        {title && <Text style={styles.title}>{title}</Text>}
        <TextInput
          style={styles.input}
          defaultValue={defaultValue}
          value={value}
          multiline
          numberOfLines={6}
          onChangeText={this.onChangeText}
          onEndEditing={(evt) => {
            if (onChange) onChange(evt.nativeEvent.text);
          }}
          underlineColorAndroid="transparent"
          onFocus={onFocus}
          placeholder={placeholder}
          autoFocus={autoFocus}
          ref={this.setInputRef}
        />
      </View>
    );
  }
}
