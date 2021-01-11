import React, { PureComponent } from 'react';
import { TouchableWithoutFeedback, Platform, View, Text, TextInput } from 'react-native';
import { isEmpty } from 'lodash';
import TextInputMask from 'react-native-text-input-mask';

import { primaryTextColor, formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    width: '100%',
  },
  inputPart: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
  },
  verticalInputPart: {
    flex: 1,
    alignItems: 'flex-start',
  },
  title: {
    width: 150,
    fontSize: 15,
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  input: {
    flex: 1,
    fontSize: 15,
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    paddingVertical: (Platform.OS === 'ios' ? 20 : 12),
    marginTop: (Platform.OS === 'ios' ? 0 : 8),
  },
  verticalInput: {
    fontSize: 15,
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    paddingTop: 12,
    paddingBottom: 12,
    paddingHorizontal: 8,
    marginTop: 6,
    borderWidth: 1,
    borderColor: '#e8e8e8',
    alignSelf: 'stretch',
    height: 49,
    borderRadius: 2,
  },
  emptyStyle: {
  },
  multilineStyle: {
    height: 125,
    paddingTop: 10,
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderColor: '#e8e8e8',
  },
  unit: {
    fontSize: 14,
    color: '#8a8a8f',
  },
};

export default class TextField extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      value: props.value || props.defaultValue || '',
      resetInput: false,
    };
  }

  componentDidUpdate(prevProps, prevState, valueUpdated) {
    if (valueUpdated) {
      this.setState({ value: this.props.value });
    }
  }

  onChangeText = (text, originText) => {
    const { onChangeText, mask } = this.props;
    const { resetInput } = this.state;
    if (isEmpty(mask)) {
      if (onChangeText) {
        onChangeText(text);
      }
      this.setState({ value: text });
    } else {
      if (onChangeText) {
        onChangeText(text, originText);
      }
      if (originText !== '') {
        this.setState({ value: text });
      } else {
        this.setState({ value: '', resetInput: !resetInput }, () => {
          setTimeout(() => {
            if (this.input) {
              this.input.focus();
            }
          }, 500);
        });
      }
    }
  };

  onEndEditing = (evt) => {
    const { onChange } = this.props;
    if (onChange) { onChange(evt.nativeEvent.text); }
  };

  setInputRef = (ref) => { this.input = ref; }

  getSnapshotBeforeUpdate(preProps, prevState) {
    if (prevState.value !== this.props.value) {
      return true;
    }
    return false;
  }

  focus = () => {
    if (this.input.focus) {
      this.input.focus();
    }
  };

  render() {
    const {
      title,
      onChange,
      noBorder,
      onSubmit,
      defaultValue,
      keyboardType,
      required,
      placeholder,
      titleWidth,
      multiline,
      noTitle,
      unit,
      onFocus,
      mask,
      editable,
      autoFocus,
      maxLength,
      style,
      vertical,
      wrapperStyle,
      secureTextEntry,
      placeholderTextColor,
    } = this.props;
    const { value, resetInput } = this.state;
    return (
      <TouchableWithoutFeedback
        activeOpacity={1}
        onPress={this.focus}
      >
        <View style={[styles.wrapper, !noBorder && !vertical && styles.borderBottom, wrapperStyle]}>
          <View style={vertical ? styles.verticalInputPart : styles.inputPart}>
            {!noTitle && (
              <Text style={[styles.title, titleWidth ? { width: titleWidth } : {}]}>
                {title}
                <Text style={required ? { color: 'red' } : {}}>{required ? '*' : ''}</Text>
              </Text>
            )}
            {mask ? (
              <TextInputMask
                refInput={this.setInputRef}
                onChangeText={this.onChangeText}
                editable={editable === undefined ? true : editable}
                style={[styles.input, (isEmpty(value) && styles.emptyStyle) || {}]}
                placeholder={placeholder}
                defaultValue={defaultValue}
                mask={mask}
                value={value}
                keyboardType={keyboardType || 'default'}
                textAlignVertical="top"
                autoFocus={autoFocus}
                maxLength={maxLength}
                // key={resetInput ? 'reset' : 'input'}
                placeholderTextColor={placeholderTextColor}
              />
            ) : (
              <TextInput
                ref={this.setInputRef}
                multiline={multiline}
                style={[
                  vertical ? styles.verticalInput : styles.input,
                  isEmpty(value) ? styles.emptyStyle : {},
                  multiline ? styles.multilineStyle : {},
                  style,
                ]}
                defaultValue={defaultValue}
                keyboardType={keyboardType || 'default'}
                value={value}
                onChange={onChange}
                onSubmitEditing={onSubmit}
                onChangeText={this.onChangeText}
                onEndEditing={this.onEndEditing}
                underlineColorAndroid="transparent"
                placeholder={placeholder}
                onFocus={onFocus}
                textAlignVertical="top"
                autoFocus={autoFocus}
                maxLength={maxLength}
                secureTextEntry={secureTextEntry}
                placeholderTextColor={placeholderTextColor}
              />
            )}
          </View>
          {unit && <Text style={styles.unit}>{unit}</Text>}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}
