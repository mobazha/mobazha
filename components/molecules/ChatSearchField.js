import React, { PureComponent } from 'react';
import { View, TouchableWithoutFeedback, TextInput } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { isEmpty } from 'lodash';

import { borderColor, primaryTextColor, mainBorderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    marginVertical: 3,
    backgroundColor: 'rgb(248, 247, 247)',
    borderRadius: 20,
  },
  wrapperWithBorder: {
    backgroundColor: 'white',
    borderWidth: 1,
    borderColor,
  },
  text: {
    flex: 1,
    height: 32,
    paddingLeft: 15,
    paddingVertical: 0,
    color: primaryTextColor,
  },
  leftIcon: {
    marginLeft: 10,
    color: 'white',
  },
  rightIcon: {
    padding: 10,
  },
  clearBtn: {
    marginRight: 5,
    marginTop: 2,
    paddingHorizontal: 10,
  },
};

class ChartSearchField extends PureComponent {
  setRef = (ref) => {
    this.searchInput = ref;
  }

  resetKeyword = () => {
    const { onChange, resetKeyword } = this.props;
    onChange('');
    resetKeyword();
    this.searchInput.focus();
  };

  render() {
    const {
      autoFocus,
      value,
      onChange,
      onFocus,
      doSearch,
      placeholder,
    } = this.props;

    return (
      <View style={styles.wrapper}>
        <TextInput
          ref={this.setRef}
          style={styles.text}
          value={value}
          onChangeText={onChange}
          onSubmitEditing={doSearch}
          onFocus={onFocus}
          underlineColorAndroid="transparent"
          autoFocus={autoFocus}
          placeholder={placeholder}
          placeholderTextColor={mainBorderColor}
        />
        {!isEmpty(value) && (
          <TouchableWithoutFeedback onPress={this.resetKeyword}>
            <View style={styles.clearBtn}>
              <Ionicons name="md-close" color={primaryTextColor} size={24} />
            </View>
          </TouchableWithoutFeedback>
        )}
      </View>
    );
  }
}

export default ChartSearchField;
