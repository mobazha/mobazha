import React, { PureComponent } from 'react';
import { Alert, View, Text, TouchableWithoutFeedback } from 'react-native';

import { formLabelColor, foregroundColor, brandColor, borderColor, greenTintColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    paddingHorizontal: 16,
    flexDirection: 'row',
  },
  optionItem: {
    height: 32,
    borderRadius: 20,
    paddingHorizontal: 16,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#F8F7F7',
    borderWidth: 1,
    borderColor: '#F8F7F7',
    marginHorizontal: 5,
    marginVertical: 16,
  },
  selectedOption: {
    height: 32,
    borderWidth: 1,
    borderColor: brandColor,
    backgroundColor: '#F2FBF3',
  },
  optionText: {
    fontSize: 14,
    fontStyle: 'normal',
    letterSpacing: 0,
    fontWeight: 'bold',
    color: formLabelColor,
  },
  selectedText: {
    color: brandColor,
  },
  disabledText: {
    color: '#c8c7cc',
    fontWeight: 'bold',
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderBottomColor: borderColor,
  },
};

export default class Options extends PureComponent {
  constructor(props) {
    super(props);
    const { currentOption } = props;
    this.state = { currentOption };
  }

  handlePress = item => () => {
    if (item.disabledMessage) {
      Alert.alert('Oops!', item.disabledMessage);
      return;
    }
    this.setState({ currentOption: item.value });
    this.props.onChange(item.value);
  }

  render() {
    const { options, withBorder } = this.props;
    const { currentOption } = this.state;
    return (
      <View style={[styles.wrapper, withBorder && styles.borderBottom]}>
        {options.map((item, idx) => (
          <TouchableWithoutFeedback onPress={this.handlePress(item)} key={idx}>
            <View
              style={[
                styles.optionItem,
                item.value === currentOption && styles.selectedOption,
              ]}
            >
              <Text
                style={[
                  styles.optionText,
                  item.value === currentOption && styles.selectedText,
                  item.disabledMessage && styles.disabledText,
                ]}
              >
                {item.label}
              </Text>
            </View>
          </TouchableWithoutFeedback>
        ))}
      </View>
    );
  }
}
