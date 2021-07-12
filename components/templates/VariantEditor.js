import React from 'react';
import { Alert, FlatList, View, findNodeHandle } from 'react-native';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';

import MoreButton from '../atoms/MoreButton';
import SingleVariantEditor from '../organism/SingleVariantEditor';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  moreButtonWrapper: {
    paddingVertical: 20,
    paddingLeft: 15,
  },
};

const isEmptyOption = ({ name, description, variants }) => {
  if (name === '' && description === '' && variants.length === 0) {
    return true;
  }
  return false;
};

export default class VaraintEditor extends React.Component {
  constructor() {
    super()
    this.scrollToEnd = this.scrollToEnd.bind(this);
  }

  handleAddMore = () => {
    this.props.addVariant();
    setTimeout(this.scrollToEnd, 300);
  }

  handleChangeOption = (index, option) => {
    const options = [...this.props.options];
    options[index] = option;
    this.props.onChangeOptions(options);
  };

  getScrollRef = (ref) => {
    this.scroll = ref;
  };

  focusInput = (evt) => {
    this.scroll.props.scrollToFocusedInput(findNodeHandle(evt.target));
  };

  scrollToEnd() {
    if (this.scroll) {
      this.scroll.scrollToEnd({ animated: true });
    }
  }

  removeOption = (index) => {
    const { options } = this.props;
    if (isEmptyOption(options[index])) {
      this.doRemove(index);
    } else {
      Alert.alert(I18n.t('components.templates.VariantEditor.Delete_variant'), I18n.t('components.templates.VariantEditor.cannot_undo'), [
        { text: I18n.t('components.templates.VariantEditor.cancel') },
        { text: I18n.t('components.templates.VariantEditor.delete'), onPress: () => this.doRemove(index) },
      ]);
    }
  };

  doRemove = (index) => {
    const { options } = this.props;
    eventTracker.trackEvent('CreateListing-DeleteVariant');
    this.props.onChangeOptions([
      ...options.slice(0, index),
      ...options.slice(index + 1, options.length),
    ]);
  }

  keyExtractor = (item, index) => `variant_${index}`;

  renderItem = ({ item, index }) => (
    <SingleVariantEditor
      option={item}
      index={index}
      onChange={this.handleChangeOption}
      removeOption={this.removeOption}
      focusInput={this.focusInput}
    />
  )

  renderListFooter = () => (
    <View style={styles.moreButtonWrapper}>
      <MoreButton title={I18n.t('components.templates.VariantEditor.Add_variant')} onPress={this.handleAddMore} />
    </View>
  );

  render() {
    const { options } = this.props;
    return (
      <KeyboardAwareScrollView
        innerRef={this.getScrollRef}
      >
        <FlatList
          ref={this.setListRef}
          data={options}
          renderItem={this.renderItem}
          keyExtractor={this.keyExtractor}
          ListFooterComponent={this.renderListFooter}
          extraData={options}
        />
      </KeyboardAwareScrollView>
    );
  }
}
