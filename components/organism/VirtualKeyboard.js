

import React, { Component } from 'react';
import PropTypes from 'prop-types';
import {
  Text,
  View,
  TouchableOpacity,
  Image,
  ViewPropTypes,
} from 'react-native';
import Feather from 'react-native-vector-icons/Feather';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    alignItems: 'flex-start',
  },
  row: {
    flexDirection: 'row',
    height: 56,
  },
  number: {
    fontSize: 25,
    textAlign: 'center',
    fontWeight: '600',
  },
  backspace: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  cell: {
    flex: 1,
    justifyContent: 'center',
  },
};

class VirtualKeyboard extends Component {
  static propTypes = {
    pressMode: PropTypes.oneOf(['string', 'char']),
    color: PropTypes.string,
    onPress: PropTypes.func.isRequired,
    decimal: PropTypes.bool,
    rowStyle: ViewPropTypes.style,
    cellStyle: ViewPropTypes.style,
    decimalPoints: PropTypes.number.isRequired,
  }

  static defaultProps = {
    pressMode: 'string',
    color: 'gray',
    decimal: false,
    rowStyle: undefined,
    cellStyle: undefined,
  }

  state = {
    text: '',
  }

  getCurrentRegex = () => {
    const { decimalPoints } = this.props;
    return new RegExp(`^[0-9]{1,13}([.][0-9]{0,${decimalPoints}})?$`);
  }

  setText = (text) => {
    this.setState({ text });
  }

  handlePress = (val) => {
    const { pressMode, onPress } = this.props;
    if (pressMode === 'string') {
      let curText = this.state.text;
      if (isNaN(val)) {
        if (val === 'back') {
          curText = curText.slice(0, -1);
        } else {
          curText += val;
        }
      } else {
        curText += val;
      }

      if (curText === '.') {
        this.setState({ text: '0.' });
        onPress('0.');
      } else if (curText === '' || this.getCurrentRegex().test(curText)) {
        this.setState({ text: curText });
        onPress(curText);
      } else {
        console.warn('invalid text', curText);
      }
    } else /* if (pressMode == 'char') */ {
      onPress(val);
    }
  }

  renderRow = numbersArray => (
    <View style={[styles.row, this.props.rowStyle]}>
      {numbersArray.map(this.renderCell)}
    </View>
  )

  renderCell = (symbol) => {
    const { cellStyle, color } = this.props;
    return (
      <TouchableOpacity
        style={[styles.cell, cellStyle]}
        key={symbol}
        accessibilityLabel={symbol.toString()}
        onPress={() => this.handlePress(symbol.toString())}
      >
        <Text style={[styles.number, { color }]}>{symbol}</Text>
      </TouchableOpacity>
    );
  }

  renderBackspace = () => {
    const { color } = this.props;
    return (
      <TouchableOpacity
        accessibilityLabel="backspace"
        style={styles.backspace}
        onPress={() => this.handlePress('back')}
      >
        <Feather name="delete" size={23} color={color} />
      </TouchableOpacity>
    );
  }

  render() {
    const { style, decimal, rowStyle } = this.props;
    return (
      <View style={[styles.container, style]}>
        {this.renderRow([1, 2, 3])}
        {this.renderRow([4, 5, 6])}
        {this.renderRow([7, 8, 9])}
        <View style={[styles.row, rowStyle]}>
          {decimal ? this.renderCell('.') : <View style={{ flex: 1 }} /> }
          {this.renderCell(0)}
          {this.renderBackspace()}
        </View>
      </View>
    );
  }
}

export default VirtualKeyboard;
