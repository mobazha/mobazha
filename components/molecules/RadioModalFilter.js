import React, { PureComponent } from 'react';
import { TouchableWithoutFeedback, Text, View, FlatList, Image, TextInput, Alert } from 'react-native';
import MaterialCommunityIcons from 'react-native-vector-icons/MaterialCommunityIcons';
import * as _ from 'lodash';

import { foregroundColor, greenColor, primaryTextColor, linkTextColor, borderColor, formLabelColor, staticLabelColor, disabledButtonTextColor } from '../commonColors';

import Section from './Section';
import OptionGroup from '../atoms/OptionGroup';
import { OBLightModal } from '../templates/OBModal';
import Header from '../molecules/Header';
import NavCloseButton from '../atoms/NavCloseButton';
import LinkText from '../atoms/LinkText';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
  },
  body: {
    paddingHorizontal: 14,
  },
  itemWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 14,
  },
  labelWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
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
    paddingVertical: 20,
    paddingRight: 25,
  },
  selectedText: {
    fontWeight: 'bold',
    color: linkTextColor,
  },
  disabledText: {
    color: disabledButtonTextColor,
  },
  itemText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  label: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: linkTextColor,
  },
  border: {
    borderBottomWidth: 1,
    borderColor,
  },
  optionIcon: {
    width: 20,
    height: 20,
    marginRight: 7,
  },
  secondaryWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: 60,
  },
  secondaryTitle: {
    width: 150,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  triggerLabel: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    paddingVertical: 20,
    paddingRight: 15,
  },
  textInput: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    paddingTop: 0,
    paddingBottom: 0,
    textAlignVertical: 'center',
  },
};

export default class RadioModalFilter extends PureComponent {
  state = {
    showModal: false,
    selectedOption: { option: '', value: '' },
  };

  onChange = val => () => {
    const { hasSubmit, onChange } = this.props;
    if (hasSubmit) {
      this.setState({ selectedOption: { option: val, value: '' } });
    } else {
      onChange(val);
      this.hideSelector();
    }
  }

  onChangeText = option => (value) => {
    this.setState({ selectedOption: { option, value } });
  }

  onSubmit = () => {
    const { selectedOption: { option, value } } = this.state;
    if (!option || (option === 'Other' && !value)) {
      Alert.alert('Ooops!', I18n.t('components.molecules.RadioModalFilter.reason_reporting'));
      return;
    }

    const { onChange } = this.props;
    onChange(option, value);
    this.hideSelector();
  }

  onOptionFocus = option => () => {
    if (this.inputRefs[option]) {
      this.setState({ selectedOption: { option, value: this.inputRefs[option].value } });
    }
  }

  setRef = value => (ref) => {
    this.inputRefs[value] = ref;
  }

  setFocus = option => () => {
    if (this.inputRefs[option]) {
      this.inputRefs[option].focus();
      this.setState({ selectedOption: { option, value: this.inputRefs[option].value } });
    }
  }

  inputRefs = [];

  keyExtractor = (item, index) => `option_${index}`;

  generateLabel = () => {
    const {
      options, selected, valueKey, getLabel,
    } = this.props;
    const idx = _.findIndex(options, (option) => {
      const optionValue = _.isEmpty(valueKey) ? _.get(option, 'value') : _.get(option, valueKey);
      return optionValue === selected;
    });
    const label = getLabel ? getLabel(_.get(options, `[${idx}]`)) : _.get(options, `[${idx}].label`);
    const icon = _.get(options, `[${idx}].icon`);
    return { label, icon };
  }

  showSelector = () => { this.setState({ showModal: true }); }

  hideSelector = () => { this.setState({ showModal: false }); }

  isSelected = (value) => {
    const { hasSubmit, selected } = this.props;
    if (hasSubmit) {
      const { selectedOption } = this.state;
      return selectedOption.option === value;
    } else {
      return selected === value;
    }
  }

  renderItem = ({ item }) => {
    const { valueKey, getLabel } = this.props;
    const { icon, type, disabled } = item;
    const value = _.isEmpty(valueKey) ? item.value : _.get(item, valueKey);
    const label = getLabel ? getLabel(item) : item.label;
    const isSelected = this.isSelected(value);
    return (
      <TouchableWithoutFeedback onPress={type === 'textInput' ? this.setFocus(value) : this.onChange(value)} disabled={disabled}>
        <View style={styles.itemWrapper}>
          {isSelected ? (
            <MaterialCommunityIcons name="radiobox-marked" color={disabled ? disabledButtonTextColor : greenColor} size={20} style={styles.checkmark} />
          ) : (
            <MaterialCommunityIcons name="radiobox-blank" color={disabled ? disabledButtonTextColor : staticLabelColor} size={20} style={styles.checkmark} />
          )}
          {icon && <Image style={styles.optionIcon} source={icon} />}
          <View style={styles.itemLabelWrapper}>
            {type === 'textInput' ? (
              <TextInput
                ref={this.setRef(value)}
                onFocus={this.onOptionFocus(value)}
                style={styles.textInput}
                multiline
                onChangeText={this.onChangeText(value)}
                underlineColorAndroid="transparent"
                placeholder={I18n.t('components.molecules.RadioModalFilter.other')}
              />
            ) : (
              <Text
                style={[styles.itemText, isSelected && styles.selectedText, disabled && styles.disabledText]}
                numberOfLines={1}
              >
                {label}
              </Text>
            )}
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  }

  renderModalContent = () => {
    const {
      selected, options, style,
    } = this.props;
    return (
      <FlatList
        data={options}
        keyExtractor={this.keyExtractor}
        renderItem={this.renderItem}
        extraData={selected}
        style={[styles.wrapper, style]}
      />
    );
  }

  renderTrigger = () => {
    const {
      title, secondary, required, hasBorder,
    } = this.props;
    const { label, icon } = this.generateLabel();
    if (secondary) {
      return (
        <OptionGroup onPress={this.showSelector} fullWidth smallPadding>
          <View style={styles.secondaryWrapper}>
            <Text style={styles.secondaryTitle} numberOfLines={1}>
              {title}
              <Text style={required ? { color: 'red' } : {}}>{required ? '*' : ''}</Text>
            </Text>
            <Text style={styles.triggerLabel} numberOfLines={1}>{label}</Text>
          </View>
        </OptionGroup>
      );
    }
    return (
      <Section title={secondary ? '' : title} bodyStyle={[styles.body, hasBorder ? styles.border : {}]}>
        <OptionGroup onPress={this.showSelector} noBorder>
          <View style={styles.labelWrapper}>
            {icon && <Image style={styles.optionIcon} source={icon} />}
            <View style={styles.itemLabelWrapper}><Text style={styles.label}>{label}</Text></View>
          </View>
        </OptionGroup>
      </Section>
    );
  }

  renderRight = () => <LinkText text="Submit" color={linkTextColor} />

  render() {
    const { hasSubmit, modalMode } = this.props;
    const { showModal } = this.state;
    return (
      <React.Fragment>
        {!modalMode && this.renderTrigger()}
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={this.hideSelector}
        >
          <Header
            modal
            title=""
            left={<NavCloseButton />}
            onLeft={this.hideSelector}
            right={hasSubmit && this.renderRight()}
            onRight={hasSubmit && this.onSubmit}
            noBorder
          />
          {this.renderModalContent()}
        </OBLightModal>
      </React.Fragment>
    );
  }
}
