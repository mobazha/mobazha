import React, { PureComponent } from 'react';
import { FlatList } from 'react-native';

import Header from '../molecules/Header';

import { OBLightModal } from '../templates/OBModal';
import CoinListItem from '../molecules/CoinListItem';
import NavCloseButton from '../atoms/NavCloseButton';

import {I18n} from '../../langs/I18n';

const WYRE_SUPPORTING_COINS = ['BTC', 'BCH', 'LTC'];

export default class CoinSelectorModal extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      coin: props.coin,
      showModal: false,
    };
  }

  onClose = () => {
    this.setState({ showModal: false });
  };

  keyExtractor = (item, index) => `coin_item_${index}`

  showModal() {
    this.setState({ showModal: true });
  }

  handleCoinSelected = (coin) => {
    this.setState(
      { coin, showModal: false },
      () => {
        this.props.onChange(coin);
      },
    );
  }

  renderItem = ({ item }) => {
    const { coin } = this.state;
    const childProps = {
      coin: item,
      onCoinSelected: this.handleCoinSelected,
    };
    return (
      <CoinListItem {...childProps} selected={coin === item} />
    );
  }

  render() {
    const { showModal } = this.state;

    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={showModal}
        onRequestClose={this.onClose}
      >
        <Header
          modal
          left={<NavCloseButton />}
          onLeft={() => this.setState({ showModal: false })}
        />
        <FlatList
          keyExtractor={this.keyExtractor}
          data={WYRE_SUPPORTING_COINS}
          renderItem={this.renderItem}
        />
      </OBLightModal>
    );
  }
}
