import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import InputTemplate from '../components/templates/ListingAdvancedDetails';

import { setDetails } from '../reducers/createListing';
import NavBackButton from '../components/atoms/NavBackButton';

import {I18n} from '../langs/I18n';

const style = {
  backgroundColor: '#FFF',
  flex: 1,
};

class ListingAdvancedDetails extends PureComponent {
  constructor(props) {
    super(props);
    const { termsAndConditions, refundPolicy } = props;
    this.state = {
      termsAndConditions,
      refundPolicy,
    };
  }

  onLeft = () => {
    this.props.navigation.goBack();
  };

  onRight = () => {
    this.props.setDetails({
      ...this.state,
    });
    this.props.navigation.goBack();
  };

  onChange = ({ termsAndConditions, refundPolicy }) => {
    this.setState({
      termsAndConditions,
      refundPolicy,
    });
  };

  render() {
    const { storeRefunds, storeTAndC, step } = this.props;
    return (
      <View style={style}>
        <Header
          left={<NavBackButton />}
          onLeft={this.onLeft}
          title={I18n.t('screens.listingAdvancedDetails.store_policies')}
          right={<LinkText text={I18n.t('screens.listingAdvancedDetails.apply')} />}
          onRight={this.onRight}
        />
        <InputTemplate
          step={step}
          details={{
            ...this.state,
            storeRefunds,
            storeTAndC,
          }}
          onChange={this.onChange}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  step: state.createListing.step,
  tags: state.createListing.tags,
  condition: state.createListing.condition,
  categories: state.createListing.categories,
  termsAndConditions: state.createListing.termsAndConditions,
  refundPolicy: state.createListing.refundPolicy,
  storeTAndC: state.settings.termsAndConditions,
  storeRefunds: state.settings.refundPolicy,
});

const mapDispatchToProps = {
  setDetails,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ListingAdvancedDetails);
