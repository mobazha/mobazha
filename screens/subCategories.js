import React, { PureComponent } from 'react';
import { View, Alert } from 'react-native';
import * as _ from 'lodash';

import { screenWrapper } from '../utils/styles';
import categories from '../config/categories';

import RadioFilter from '../components/molecules/RadioFilter';
import { eventTracker } from '../utils/EventTracker';
import NavBackButton from '../components/atoms/NavBackButton';
import LinkText from '../components/atoms/LinkText';
import Header from '../components/molecules/Header';

import {I18n} from '../langs/I18n';
class SubCategories extends PureComponent {
  constructor(props) {
    super(props);

    const { navigation } = props;
    const { name } = navigation.getParam('item');
    const categoryObject = categories.find(category => category.name === name);
    const subs = (categoryObject && categoryObject.subs) || [];
    this.subOptions = subs.map(sub => ({ value: sub, label: sub }));
  }

  state = { option: '' };

  handleChangeOption = (option) => { this.setState({ option }); };

  handleGoBack = () => {
    this.props.navigation.goBack();
  };

  handleDone = () => {
    const { navigation } = this.props;
    const { name } = navigation.getParam('item');
    const onChangeCategory = navigation.getParam('onChangeCategory');

    const { option } = this.state;
    if (_.isEmpty(option)) {
      Alert.alert('Oops!', 'Please select a subcategory');
      return;
    }

    onChangeCategory(name, option);
    navigation.pop(2);
  }

  render() {
    const { option } = this.state;

    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.handleGoBack}
          right={<LinkText text="Done" />}
          onRight={this.handleDone}
        />
        <RadioFilter
          wrapperStyle={{ flex: 1 }}
          style={{ flex: 1 }}
          title="Choose a subcategory"
          selected={option}
          options={this.subOptions}
          onChange={this.handleChangeOption}
        />
      </View>
    );
  }
}

export default SubCategories;
