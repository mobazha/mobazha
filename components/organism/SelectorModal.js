import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback, ScrollView } from 'react-native';
import FontAwesome from 'react-native-vector-icons/FontAwesome';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { findIndex } from 'lodash';

import Header from '../molecules/Header';
import { formLabelColor, primaryTextColor, borderColor, brandColor, staticLabelColor } from '../commonColors';

import NavCloseButton from '../atoms/NavCloseButton';
import { OBLightModal } from '../templates/OBModal';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    height: 55,
    justifyContent: 'center',
  },
  verticalWrapper: {
    height: 75,
  },
  border: {
    borderWidth: 1,
    borderColor,
    borderRadius: 2,
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderColor,
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
  content: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  input: {
    fontSize: 15,
    lineHeight: 15,
    textAlign: 'left',
    color: primaryTextColor,
    marginVertical: 15,
    marginRight: 15,
  },
  verticalInput: {
    fontSize: 15,
    lineHeight: 15,
    textAlign: 'left',
    color: primaryTextColor,
    marginVertical: 16,
    marginLeft: 8,
  },
  underline: {
    textDecorationLine: 'underline',
  },
  optionTrigger: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  verticalOptionTrigger: {
    alignItems: 'flex-start',
    justifyContent: 'space-between',
  },
  option: {
    padding: 16,
  },
  selected: {
    color: brandColor,
  },
  optionText: {
    color: primaryTextColor,
    fontSize: 16,
  },
  iconDown: {
    paddingRight: 12,
  },
  iconForward: {
    paddingRight: 0,
  },
  small: {
    fontSize: 12,
    color: '#BEEED7',
  },
};

export default class SelectorModal extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      selection: props.value,
      showModal: false,
    };
  }

  onClose = () => {
    this.setState({ showModal: false });
  };

  onSelect(val) {
    this.setState(
      { selection: val, showModal: false },
      () => {
        this.props.onChange(val);
      },
    );
  }

  getOptionLabel(val) {
    const {
      options, valueKey, getTitleLabel, getLabel,
    } = this.props;
    const idx = findIndex(options, o => o[valueKey] === val);
    return idx >= 0 ? (
      getTitleLabel ? getTitleLabel(options[idx]) : getLabel(options[idx])
    ) : (
      I18n.t('components.organism.SelectorModal.none')
    );
  }

  render() {
    const { selection, showModal } = this.state;
    const {
      options,
      title,
      valueKey,
      getLabel,
      getTitleLabel,
      noBorder,
      titleWidth,
      required,
      compact,
      white,
      small,
      vertical,
      wrapperStyle,
      inputStyle,
    } = this.props;

    let text = `Select ${title}`;
    if (selection) {
      if (selection[valueKey]) {
        text = getTitleLabel ? getTitleLabel(selection) : getLabel(selection);
      } else {
        text = this.getOptionLabel(selection);
      }
    }

    return (
      <View
        style={[
          vertical ? styles.verticalWrapper : styles.wrapper,
          compact && { alignSelf: 'center' },
          wrapperStyle,
        ]}
      >
        <TouchableWithoutFeedback
          onPress={() => {
            this.setState({ showModal: true });
          }}
        >
          <View
            style={[
              vertical ? styles.verticalOptionTrigger : styles.optionTrigger,
              !noBorder && !vertical && styles.borderBottom,
              !compact && { flex: 1 },
            ]}
          >
            <Text
              style={[
                styles.title,
                titleWidth ? { width: titleWidth } : (compact && { width: undefined }),
                white && { color: 'white' },
                small && styles.small,
              ]}
            >
              {title}
              <Text style={required ? { color: 'red' } : {}}>{required ? '*' : ''}</Text>
            </Text>
            <View style={[styles.content, !compact && !vertical && { flex: 1 }, vertical && styles.border]}>
              <Text
                style={[
                  vertical ? styles.verticalInput : styles.input,
                  compact ? styles.underline : { flex: 1 },
                  white && { color: 'white' },
                  small && styles.small,
                  inputStyle,
                ]}
                numberOfLines={1}
                ellipsizeMode="tail"
              >
                {text}
              </Text>
              {vertical ? (
                <FontAwesome
                  style={styles.iconDown}
                  name="sort"
                  size={18}
                  color={brandColor}
                />
              ) : !compact && (
                <Entypo
                  name="chevron-thin-right"
                  size={18}
                  color={primaryTextColor}
                  style={styles.iconForward}
                />
              )}
            </View>
          </View>
        </TouchableWithoutFeedback>
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={this.onClose}
        >
          <Header modal left={<NavCloseButton />} onLeft={this.onClose} />
          <ScrollView style={{ flex: 1 }}>
            {options.map((val, idx) => {
              const isSelected =
                val[valueKey] === selection[valueKey] || val[valueKey] === selection;
              return (
                <TouchableWithoutFeedback
                  key={idx}
                  onPress={() => {
                    this.onSelect(val);
                  }}
                >
                  <View style={styles.option}>
                    <Text style={[styles.optionText, isSelected ? styles.selected : null]}>
                      {getLabel(val)}
                    </Text>
                  </View>
                </TouchableWithoutFeedback>
              );
            })}
          </ScrollView>
        </OBLightModal>
      </View>
    );
  }
}

SelectorModal.defaultProps = {
  valueKey: 'value',
  getLabel: option => option.label,
};
