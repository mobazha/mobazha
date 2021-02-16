import React, { PureComponent } from 'react';
import { Animated } from 'react-native';
import { isEmpty } from 'lodash';

import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';
import ModerationFee from '../molecules/ModerationFee';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingTop: 30,
  },
};

export default class ModerationSettingsEditor extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      description: isEmpty(props.description) ? '' : props.description,
      termsAndConditions: isEmpty(props.termsAndConditions) ? '' : props.termsAndConditions,
      languages: isEmpty(props.languages) ? [] : props.languages,
      fee: isEmpty(props.fee) ? props.fee : {},
      aniVal: new Animated.Value(0),
    };
  }

  UNSAFE_componentWillMount() {
    Animated.timing(
      this.state.aniVal,
      {
        duration: 1000,
        toValue: 1,
      },
    ).start();
  }

  onChange() {
    const {
      description, termsAndConditions, languages, fee,
    } = this.state;
    this.props.onChange({
      description, termsAndConditions, languages, fee,
    });
  }

  setLanguage(lang, idx) {
    const languages = [...this.state.languages];
    languages[idx] = lang;
    this.setState({
      languages,
    }, () => {
      this.onChange();
    });
  }

  render() {
    const {
      description, termsAndConditions, languages, fee,
    } = this.state;
    return (
      <Animated.View
        style={[
          styles.wrapper,
          {
            opacity: this.state.aniVal,
            marginTop: this.state.aniVal.interpolate({
              inputRange: [0, 1],
              outputRange: [0, -25],
            }),
          },
        ]}
      >
        <InputGroup title="Profile Information">
          <TextInput
            title="Description"
            required
            value={description}
            onChangeText={(description) => {
              this.setState({
                description,
              }, () => {
                this.onChange();
              });
            }}
          />
          <TextInput
            title="Terms of Service"
            required
            noBorder
            value={termsAndConditions}
            onChangeText={(termsAndConditions) => {
              this.setState({
                termsAndConditions,
              }, () => {
                this.onChange();
              });
            }}
          />
        </InputGroup>
        <InputGroup title="Languages">
          <TextInput
            title="Primary"
            value={languages[0] ? languages[0] : ''}
            onChangeText={(text) => {
              this.setLanguage(text, 0);
            }}
          />
          <TextInput
            title="Secondary"
            value={languages[1] ? languages[1] : ''}
            onChangeText={(text) => {
              this.setLanguage(text, 1);
            }}
          />
          <TextInput
            title="Third"
            noBorder
            value={languages[2] ? languages[2] : ''}
            onChangeText={(text) => {
              this.setLanguage(text, 2);
            }}
          />
        </InputGroup>
        <ModerationFee
          fee={fee}
          onChange={(fee) => {
            this.setState({
              fee,
            }, () => {
              this.onChange();
            });
          }}
        />
      </Animated.View>
    );
  }
}
