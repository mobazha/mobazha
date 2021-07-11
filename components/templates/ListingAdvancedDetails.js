import React, { PureComponent } from 'react';
import { Animated } from 'react-native';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';

import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';

import {I18n} from '../../langs/I18n';
class InputTemplate extends PureComponent {
  constructor(props) {
    super(props);
    const { details } = props;
    this.state = {
      termsAndConditions: details.termsAndConditions,
      storewideTerm: false,
      refundPolicy: details.refundPolicy,
      storewideRefund: false,
    };
  }

  UNSAFE_componentWillMount() {
    const {
      details: { storeRefunds, storeTAndC },
    } = this.props;
    const { termsAndConditions, refundPolicy } = this.state;
    const storewideTerm = termsAndConditions === '' ? false : termsAndConditions === storeTAndC;
    const storewideRefund = refundPolicy === '' ? false : refundPolicy === storeRefunds;
    if (!storewideTerm) {
      this.termAnival.setValue(1);
    }
    if (!storewideRefund) {
      this.refundAniVal.setValue(1);
    }
    this.setState({
      storewideTerm,
      storewideRefund,
      termsAndConditions: termsAndConditions === '' ? storeTAndC : termsAndConditions,
      refundPolicy: refundPolicy === '' ? storeRefunds : refundPolicy,
    });
  }

  onChangeRefundPolicy = (text) => {
    const { onChange } = this.props;
    this.setState(
      {
        refundPolicy: text,
      },
      () => {
        onChange(this.state);
      },
    );
  };

  onChangeTCPolicy = (text) => {
    const { onChange } = this.props;
    this.setState(
      {
        termsAndConditions: text,
      },
      () => {
        onChange(this.state);
      },
    );
  };

  refundAniVal = new Animated.Value(0);

  termAnival = new Animated.Value(0);

  render() {
    const {
      termsAndConditions,
      refundPolicy,
    } = this.state;
    return (
      <KeyboardAwareScrollView>
        <InputGroup title={I18n.t('components.templates.ListingAdvancedDetails.Return_Policy')}>
          <Animated.View
            style={{
              overflow: 'hidden',
              height: this.refundAniVal.interpolate({
                inputRange: [0, 1],
                outputRange: [50, 193],
              }),
            }}
          >
            <TextInput
              title="Refunds"
              noBorder
              noTitle
              multiline
              value={refundPolicy}
              onChangeText={this.onChangeRefundPolicy}
              placeholder={I18n.t('components.templates.ListingAdvancedDetails.return_description')}
            />
          </Animated.View>
        </InputGroup>
        <InputGroup title={I18n.t('components.templates.ListingAdvancedDetails.terms')}>
          <Animated.View
            style={{
              overflow: 'hidden',
              height: this.termAnival.interpolate({
                inputRange: [0, 1],
                outputRange: [50, 193],
              }),
            }}
          >
            <TextInput
              title={I18n.t('components.templates.ListingAdvancedDetails.T_C')}
              noBorder
              noTitle
              multiline
              value={termsAndConditions}
              onChangeText={this.onChangeTCPolicy}
              placeholder={I18n.t('components.templates.ListingAdvancedDetails.terms_description')}
            />
          </Animated.View>
        </InputGroup>
      </KeyboardAwareScrollView>
    );
  }
}

export default InputTemplate;
