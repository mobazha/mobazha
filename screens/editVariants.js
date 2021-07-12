import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert } from 'react-native';
import { NavigationEvents } from 'react-navigation';
import { isEmpty } from 'lodash';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import VariantEditor from '../components/templates/VariantEditor';
import { screenWrapper } from '../utils/styles';

import { updateOptions } from '../reducers/createListing';
import NavBackButton from '../components/atoms/NavBackButton';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';
class EditVariants extends PureComponent {
  constructor(props) {
    super(props);
    const options = [...props.options];
    if (options.length === 0) {
      options.push({
        name: '',
        variants: [],
        description: '',
      });
    }
    this.state = {
      options,
    };
  }

  setVariantEditorRef = (ref) => { this.variantEditor = ref; }

  hasOption = () => {
    const { options } = this.state;
    if (options.length > 0) {
      const { name, description, variants = [] } = options[0];
      return !(name === '' && description === '' && variants.length === 0);
    }
    return false;
  }

  handleChangeOptions = (options) => {
    this.setState({ options });
  };

  handleBack = () => {
    Alert.alert(I18n.t('screens.editVariants.are_you_sure'), I18n.t('screens.editVariants.unsaved_discard'), [
      { text: I18n.t('screens.editVariants.cancel') },
      { text: I18n.t('screens.editVariants.ok'), onPress: this.props.navigation.goBack },
    ]);
  };

  handleSave = () => {
    const { options } = this.state;
    const { updateOptions, navigation } = this.props;

    const errorMessage = this.validateOptions();
    if (!errorMessage) {
      updateOptions(options);
      navigation.goBack();
    } else {
      Alert.alert(errorMessage);
    }
  };

  handleNewVariant = () => {
    const options = [...this.state.options];
    options.push({
      name: '',
      variants: [],
      description: '',
    });
    this.setState({ options });
    eventTracker.trackEvent('CreateListing-CreatedVariant');
  };

  handleNavigationFocus = () => {
    const { options } = this.state;
    const { navigation } = this.props;
    const editing = navigation.getParam('editing');
    if (!editing && options.length > 0) {
      if (this.hasOption()) {
        this.handleNewVariant();
      }
      if (this.variantEditor) {
        setTimeout(() => {
          this.variantEditor.scrollToEnd();
        }, 100);
      }
    }
  }

  // This returns null if all the options are valid, otherwise it returns error message
  validateOptions = () => {
    const { options } = this.state;
    if (!options.length) {
      return null;
    }

    for (const option of options) {
      const { name, variants = [] } = option;
      if (name === '') {
        return I18n.t('screens.editVariants.fill_required');
      }

      if (variants.length < 2) {
        return I18n.t('screens.editVariants.fill_choices');
      }
    }

    return null;
  }

  render() {
    const { options } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <NavigationEvents onDidFocus={this.handleNavigationFocus} />
        <Header
          left={<NavBackButton />}
          onLeft={this.handleBack}
          title={I18n.t('screens.editVariants.Manage_Variants')}
          right={<LinkText text={I18n.t('screens.editVariants.Save')} />}
          onRight={this.handleSave}
        />
        <VariantEditor
          ref={this.setVariantEditorRef}
          options={options}
          onChangeOptions={this.handleChangeOptions}
          addVariant={this.handleNewVariant}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  options: state.createListing.options,
});

const mapDispatchToProps = {
  updateOptions,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(EditVariants);
