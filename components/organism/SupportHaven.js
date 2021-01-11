import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { TouchableOpacity, View, Text, Platform } from 'react-native';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import { formLabelColor, borderColor, warningColor, primaryTextColor, brandColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    alignSelf: 'stretch',
    marginHorizontal: 20,
    backgroundColor: 'white',
    alignItems: 'flex-start',
    marginBottom: ifIphoneX(48, 28),
  },
  title: {
    fontSize: 16,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  content: {
    alignSelf: 'stretch',
    marginTop: 8,
    paddingHorizontal: 14,
    borderRadius: 8,
    borderWidth: 1,
    borderColor,
    justifyContent: 'center',
  },
  description: {
    marginTop: 16,
    color: primaryTextColor,
    fontSize: 14,
    textAlign: 'center',
  },
  buttons: {
    marginTop: 16,
    marginBottom: 16,
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  item: {
    width: '30%',
    height: 34,
    borderRadius: 4,
    borderWidth: 1,
    borderColor,
    justifyContent: 'center',
    alignItems: 'center',
  },
  text: {
    fontSize: 14,
    color: brandColor,
    fontWeight: 'bold',
  },
};

class SupportHaven extends PureComponent {
  constructor(props) {
    super(props);

    this.handleAmount1Selected = this.handleAmountSelected(1);
    this.handleAmount3Selected = this.handleAmountSelected(3);
    this.handleAmount5Selected = this.handleAmountSelected(5);
  }

  handleAmountSelected = amount => () => {
    const { convertCurrencyToBCH, onAmountSelected, paymentMethod } = this.props;
    onAmountSelected(convertCurrencyToBCH(amount, paymentMethod));
  }

  render() {
    return (
      <View style={styles.wrapper}>
        <Text style={styles.title}>{I18n.t('components.organism.SupportHaven.support_haven')}</Text>
        <View style={styles.content}>
          <Text style={styles.description}>{I18n.t('components.organism.SupportHaven.description')}</Text>
          <View style={styles.buttons}>
            <TouchableOpacity style={styles.item} onPress={this.handleAmount1Selected}>
              <Text style={styles.text}>$1</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.item} onPress={this.handleAmount3Selected}>
              <Text style={styles.text}>$3</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.item} onPress={this.handleAmount5Selected}>
              <Text style={styles.text}>$5</Text>
            </TouchableOpacity>
          </View>
        </View>
      </View>
    );
  }
}


const mapStateToProps = state => ({
  paymentMethod: state.appstate.paymentMethod,
  balance: state.wallet.balance,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(SupportHaven);
