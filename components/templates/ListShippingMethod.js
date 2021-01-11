import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { FlatList, View, Alert } from 'react-native';

import MoreButton from '../atoms/MoreButton';
import ShippingMethod from '../organism/ShippingMethod';
import EmptyShippingMethods from '../organism/EmptyShippingMethods';


import OBActionSheet from '../organism/ActionSheet';

import {I18n} from '../../langs/I18n';

const actionList = ['Edit', 'Delete', 'Cancel'];

const styles = {
  wrapper: {
    flex: 1,
  },
  list: {
    flex: 1,
  },
  moreButtonWrapper: {
    paddingVertical: 20,
    paddingLeft: 15,
  },
};

class ListShippingMethod extends PureComponent {
  state = {
    selectedOption: -1,
  };

  onClickOption = (id) => {
    this.setState({
      selectedOption: id,
    });
    this.actionSheet.show();
  };

  setActionSheet = (ref) => {
    this.actionSheet = ref;
  };

  handleChange = (index) => {
    const { selectedOption } = this.state;
    switch (index) {
      case 0:
        this.props.onEdit(selectedOption);
        break;
      case 1:
        this.confirmRemove();
        break;
      default:
        break;
    }
  };

  confirmRemove = () => {
    const { selectedOption } = this.state;
    Alert.alert('Delete shipping option?', "You can't undo this action", [
      { text: 'Cancel' },
      { text: 'Delete', onPress: () => this.props.onRemove(selectedOption) },
    ]);
  };
  keyExtractor = (item, index) => `shippingOption${index}`;
  renderItem = ({ item, index }) => {
    return (
      <ShippingMethod
        pos={index}
        method={item}
        onClickOption={this.onClickOption}
      />
    );
  };
  render() {
    const { shippingOptions } = this.props;
    return (
      <React.Fragment>
        <FlatList
          contentContainerStyle={styles.list}
          data={shippingOptions}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderItem}
          ListEmptyComponent={<EmptyShippingMethods onAdd={this.props.onAdd} />}
          ListFooterComponent={
            shippingOptions.length > 0 && (
              <View style={styles.moreButtonWrapper}>
                <MoreButton title="Add shipping option" onPress={this.props.onAdd} />
              </View>
            )
          }
          extraData={shippingOptions}
        />
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={actionList}
          cancelButtonIndex={2}
        />
      </React.Fragment>
    );
  }
}

const mapStateToProps = state => ({
  shippingOptions: state.createListing.shippingOptions,
});

export default connect(mapStateToProps)(ListShippingMethod);
