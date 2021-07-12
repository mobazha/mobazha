import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback, FlatList, Image, TouchableOpacity, Keyboard } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { remove, findIndex, hasIn } from 'lodash';

import Header from '../molecules/Header';
import { formLabelColor, primaryTextColor, borderColor, greenColor, linkTextColor } from '../commonColors';
import NavBackButton from '../atoms/NavBackButton';
import { OBLightModal } from '../templates/OBModal';
import LinkText from '../atoms/LinkText';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: 55,
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
    paddingVertical: 20,
    paddingRight: 10,
    marginRight: 10,
  },
  optionTrigger: {
    flex: 1,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  selected: {
    backgroundColor: '#c8c7cc',
  },
  icon: {
    paddingRight: 19,
  },
  itemWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 14,
  },
  optionIcon: {
    width: 20,
    height: 20,
    marginRight: 7,
  },
  itemLabelWrapper: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 20,
  },
  itemText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  selectedText: {
    fontWeight: 'bold',
    color: linkTextColor,
  },
  checkmark: {
    color: greenColor,
    marginRight: 14,
    width: 14,
  },
  selectionIndicatorWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingLeft: 21,
    paddingRight: 11,
    paddingVertical: 11,
    borderTopWidth: 1,
    borderColor,
  },
  selectedIndicator: {
    fontSize: 15,
    color: primaryTextColor,
    letterSpacing: 0,
  },
};

export default class MultiSelector extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      selection: props.selection,
      showModal: false,
    };
  }

  onClose = () => {
    this.setState({ showModal: false });
  };

  onPressItem = (isSelected, option) => () => {
    if (isSelected) {
      this.removeOption(option);
    } else {
      this.addOption(option);
    }
  }

  onShowSelector = () => {
    Keyboard.dismiss();
    this.setState({ showModal: true });
  }

  addOption = (option) => {
    const { selection } = this.state;
    let newSelection = [];
    if (selection.length === 1 && selection[0] === 'ALL') {
      newSelection = [];
    } else {
      newSelection = [...selection];
    }
    if (option.value === 'ALL') {
      newSelection = [{ ...option }];
    } else {
      newSelection.push(option);
    }
    this.setState(
      {
        selection: newSelection,
      },
      () => {
        this.props.onChange(this.state.selection);
      },
    );
  };

  removeOption = (option) => {
    const newSelection = [...this.state.selection];
    remove(newSelection, (o) => {
      if (hasIn(o, 'value')) {
        return o.value === option.value;
      }
      return o === option.value;
    });
    this.setState(
      {
        selection: newSelection,
      },
      () => {
        this.props.onChange(this.state.selection);
      },
    );
  };

  checkSelected = (option) => {
    const selection = [...this.state.selection];
    const idx = findIndex(selection, (o) => {
      if (hasIn(o, 'value')) {
        return option.value === o.value;
      }
      return option.value === o;
    });
    return idx >= 0;
  };

  resetSelection = () => {
    this.setState({ selection: [] });
  }

  keyExtractor = (item, index) => `key_${index}`

  renderItem = ({ item: option }) => {
    const isSelected = this.checkSelected(option);
    const { icon, label } = option;
    return (
      <TouchableWithoutFeedback onPress={this.onPressItem(isSelected, option)}>
        <View style={styles.itemWrapper}>
          {icon && <Image style={styles.optionIcon} source={icon} />}
          <View style={styles.itemLabelWrapper}>
            <Text style={[styles.itemText, isSelected ? styles.selectedText : {}]}>
              {label}
            </Text>
          </View>
          {isSelected && <Ionicons name="md-checkmark" size={17} style={styles.checkmark} />}
        </View>
      </TouchableWithoutFeedback>
    );
  }

  render() {
    const { selection, showModal } = this.state;
    const { options, title, required } = this.props;
    let text = I18n.t('components.organism.MultiSelector.none');
    if (selection.length === 1) {
      if (selection[0] === 'ALL') {
        const idx = findIndex(options, option => option.value === 'ALL');
        if (idx >= 0) {
          text = options[idx].label;
        } else {
          text = I18n.t('components.organism.MultiSelector.all');
        }
      } else if (hasIn(selection, '[0].label')) {
        text = selection[0].label;
      } else {
        const idx = findIndex(options, option => option.value.toLowerCase() === selection[0].toLowerCase());
        text = options[idx].label;
      }
    } else if (selection.length > 1) {
      text = `${selection.length} selected`;
    }
    return (
      <View style={styles.wrapper}>
        <TouchableWithoutFeedback onPress={this.onShowSelector}>
          <View style={styles.optionTrigger}>
            <Text style={styles.title}>
              {title}
              <Text style={required ? { color: 'red' } : {}}>{required ? '*' : ''}</Text>
            </Text>
            <Text style={styles.input} numberOfLines={1} ellipsizeMode="tail">{text}</Text>
            <Ionicons
              name="ios-arrow-forward"
              size={18}
              color={primaryTextColor}
              style={styles.icon}
            />
          </View>
        </TouchableWithoutFeedback>
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={this.onClose}
        >
          <Header
            modal
            title=""
            left={<NavBackButton />}
            onLeft={this.onClose}
            right={<LinkText text={I18n.t('components.organism.MultiSelector.done')} />}
            onRight={this.onClose}
          />
          <FlatList
            keyExtractor={this.keyExtractor}
            renderItem={this.renderItem}
            data={options}
            extraData={selection}
          />
          {selection.length > 0 && (
            <View style={styles.selectionIndicatorWrapper}>
              <Text style={styles.selectedIndicator}>
                {I18n.t('components.organism.MultiSelector.select_info', {count: selection.length})} 
              </Text>
              <TouchableOpacity onPress={this.resetSelection}>
                <LinkText text={I18n.t('components.organism.MultiSelector.reset')} />
              </TouchableOpacity>
            </View>
          )}
        </OBLightModal>
      </View>
    );
  }
}
