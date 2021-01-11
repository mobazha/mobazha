import React from 'react';
import { Dimensions, KeyboardAvoidingView } from 'react-native';

import { navHeightStyle } from '../../utils/navbar';
import { StatusBarSpacer } from '../../status-bar';
import { keyboardAvoidingViewSharedProps } from '../../utils/keyboard';

import {I18n} from '../../langs/I18n';

const { height, width } = Dimensions.get('window');
const styles = {
  wrapper: {
    position: 'absolute',
    width,
    height: height - navHeightStyle.height,
    top: navHeightStyle.height,
    left: 0,
  },
};

export default props => (
  <KeyboardAvoidingView style={styles.wrapper} {...keyboardAvoidingViewSharedProps} pointerEvents="box-none">
    <StatusBarSpacer />
    {props.children}
  </KeyboardAvoidingView>
);
