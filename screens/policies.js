import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, TextInput } from 'react-native';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import InputGroup from '../components/atoms/InputGroup';
import { patchSettingsRequest } from '../reducers/settings';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';

const styles = {
  input: {
    textAlignVertical: 'top',
    minHeight: 150,
    paddingTop: 16,
    paddingBottom: 16,
  },
};

class Policies extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      refundPolicy: props.settings.refundPolicy,
      termsAndConditions: props.settings.termsAndConditions,
    };
  }

  componentDidMount() {
    this.termsInput.focus();
  }

  onLeft = () => { this.props.navigation.goBack(); };

  onRight = () => {
    this.props.patchSettingsRequest(this.state);
    this.props.navigation.goBack();
  };

  onChangeTerms = (termsAndConditions) => { this.setState({ termsAndConditions }); };

  onChangeRefund = (refundPolicy) => { this.setState({ refundPolicy }); };

  setTermsRef = (ref) => { this.termsInput = ref; };

  setRefundRef = (ref) => { this.refundInput = ref; };

  changeFocus = () => { this.refundInput.focus(); };

  render() {
    const { refundPolicy, termsAndConditions } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.onLeft}
          title={I18n.t('screens.policies.store_policies')}
          right={<LinkText text={I18n.t('screens.policies.save')} />}
          onRight={this.onRight}
        />
        <KeyboardAwareScrollView style={{ flex: 1 }}>
          <InputGroup title={I18n.t('screens.policies.terms')} >
            <TextInput
              ref={this.setTermsRef}
              style={styles.input}
              multiline
              numberOfLines={6}
              onChangeText={this.onChangeTerms}
              onSubmitEditing={this.changeFocus}
              value={termsAndConditions}
              noBorder
              underlineColorAndroid="transparent"
              placeholder={I18n.t('screens.policies.terms_hint')}
            />
          </InputGroup>
          <InputGroup title={I18n.t('screens.policies.refunds')}>
            <TextInput
              ref={this.setRefundRef}
              style={styles.input}
              multiline
              numberOfLines={6}
              onChangeText={this.onChangeRefund}
              value={refundPolicy}
              noBorder
              underlineColorAndroid="transparent"
              placeholder={I18n.t('screens.policies.refund_hint')}
            />
          </InputGroup>
        </KeyboardAwareScrollView>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  settings: state.settings,
});

const mapDispatchToProps = {
  patchSettingsRequest,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Policies);
