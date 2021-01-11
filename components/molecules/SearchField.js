import React, { PureComponent } from 'react';
import {
  View, TouchableWithoutFeedback,
  TouchableOpacity, TextInput,
  StyleSheet, Image, Platform,
} from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { isEmpty } from 'lodash';

import NavBackButton from '../atoms/NavBackButton';
import FilterButton from '../atoms/FilterButton';
import QRScanner from '../organism/QRScanner';
import { borderColor, primaryTextColor, formLabelColor } from '../commonColors';
import LogoImage from '../../assets/images/logo/splash-logo.png';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  wrapperWithBorder: {
    backgroundColor: 'white',
    borderWidth: 1,
    borderColor,
  },
  focusedWrapper: {
    backgroundColor: 'white',
    paddingBottom: 0,
  },
  focusedText: {
    backgroundColor: '#F8F7F7',
    marginRight: 10,
    flex: 1,
    height: 32,
    paddingLeft: 48,
    marginVertical: (Platform.OS === 'ios' ? 5 : 9),
    color: primaryTextColor,
    borderRadius: 20,
  },
  text: {
    backgroundColor: 'transparent',
    marginRight: 0,
    flex: 1,
    height: 32,
    paddingLeft: 10,
    paddingVertical: 0,
    color: '#FFFFFF',
    borderRadius: 2,
  },
  searchWrapper: {
    position: 'relative',
    flex: 1,
    paddingLeft: 8,
  },
  leftIcon: {
    position: 'absolute',
    top: Platform.select({ ios: 10, android: 16 }),
    left: 24,
  },
  rightIcon: {
    padding: 10,
  },
  clearBtn: {
    position: 'absolute',
    paddingHorizontal: 10,
    right: 15,
    top: (Platform.OS === 'ios' ? 9 : 15),
  },
  logoWrapper: {
    position: 'absolute',
    width: '100%',
    alignItems: 'center',
    justifyContent: 'center',
    paddingLeft: 10,
  },
  logo: {
    width: 50,
    height: 20,
  },
};

class SearchField extends PureComponent {
  constructor(props) {
    super(props);

    const { autoFocus, onClick } = props;
    this.state = { focused: autoFocus };

    this.clickCover = (
      <TouchableWithoutFeedback onPress={onClick}>
        <View style={StyleSheet.absoluteFill} />
      </TouchableWithoutFeedback>
    );
  }

  onFocus = () => {
    const { onFocus } = this.props;
    this.setState({ focused: true });
    if (onFocus) {
      onFocus();
    }
  }

  setFocus = () => {
    this.searchInput.focus();
  }

  setSearchInputRef = (ref) => {
    this.searchInput = ref;
  }

  getIconColor = () => {
    const { focused } = this.state;
    return focused ? primaryTextColor : 'white';
  }

  resetKeyword = () => {
    const { onChange } = this.props;
    onChange('');
    this.searchInput.focus();
  };


  render() {
    const {
      autoFocus,
      value,
      onChange,
      doSearch,
      onClick,
      onBack,
      hasBorder,
      style,
      placeholder,
      showQRButton,
      withLogo,
      toFilter,
      disableFilter,
    } = this.props;

    const { focused } = this.state;

    return (
      <View
        style={[
          styles.wrapper,
          hasBorder ? styles.wrapperWithBorder : null,
          style,
          !withLogo && styles.focusedWrapper,
        ]}
      >
        {(onBack || focused) && (
          <TouchableOpacity onPress={onBack}><NavBackButton white={withLogo} /></TouchableOpacity>
        )}
        {withLogo && (
          <View style={styles.logoWrapper}>
            <Image style={styles.logo} resizeMode="contain" source={LogoImage} />
          </View>
        )}
        <View style={styles.searchWrapper}>
          <TextInput
            ref={this.setSearchInputRef}
            style={withLogo ? styles.text : styles.focusedText}
            value={value}
            onChangeText={onChange}
            onSubmitEditing={doSearch}
            onFocus={this.onFocus}
            underlineColorAndroid="transparent"
            autoFocus={autoFocus}
            placeholder={withLogo ? '' : placeholder}
          />
          <Ionicons style={styles.leftIcon} color={formLabelColor} name="md-search" size={24} />
          {!isEmpty(value) && (
            <TouchableWithoutFeedback onPress={this.resetKeyword}>
              <View style={styles.clearBtn}><Ionicons name="md-close" color={primaryTextColor} size={24} /></View>
            </TouchableWithoutFeedback>
          )}
          {onBack && onClick && this.clickCover}
        </View>
        {!onBack && onClick && this.clickCover}
        {toFilter && <FilterButton disabled={disableFilter} onPress={toFilter} />}
        {showQRButton && !toFilter && <QRScanner style={styles.rightIcon} black={!withLogo} />}
      </View>
    );
  }
}

export default SearchField;
