import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text } from 'react-native';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import { brandColor } from '../commonColors';
import BuyNowButton from './FullButton';
import { priceStyle } from '../commonStyles';
import { convertorsMap } from '../../selectors/currency';
import { getFixedCurrency } from '../../utils/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'white',
    flexDirection: 'row',
    alignItems: 'center',
    borderTopWidth: 1,
    borderColor: '#e8e8e8',
    paddingBottom: ifIphoneX(34, 0),
  },
  price: {
    flex: 5,
    fontSize: 18,
    lineHeight: 18,
    fontWeight: 'bold',
    textAlign: 'left',
    marginLeft: 16,
  },
  saveButtonContainer: {
    flex: 3,
  },
  saveButton: {
    backgroundColor: brandColor,
    marginLeft: 0,
  },
};

class EditListingFooter extends PureComponent {
  render() {
    const { amount, localLabelFromLocal, onSave } = this.props;
    return (
      <View style={styles.wrapper}>
        <Text style={[priceStyle, styles.price]}>
          {localLabelFromLocal(parseFloat(amount) || 0.0)}
        </Text>
        <View style={styles.saveButtonContainer}>
          <BuyNowButton wrapperStyle={styles.saveButton} title={I18n.t('components.atoms.EditListingFooter.save')} onPress={onSave} />
        </View>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  amount: state.createListing.price,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(EditListingFooter);
