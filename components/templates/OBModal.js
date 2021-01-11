import React from 'react';
import { Modal, View } from 'react-native';

import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  black: {
    backgroundColor: 'black',
    flex: 1,
  },
  white: {
    backgroundColor: 'white',
    flex: 1,
  },
  overlay: {
    backgroundColor: 'transparent',
    flex: 1,
  },
};

// transparent - true if statusbar is transparent.
// overlay - true if modal background is transparent
export const OBLightModal = ({ children, overlay, transparent, ...props }) => (
  <Modal style={{ flex: 1 }} transparent={overlay || transparent} {...props}>
    {!overlay && (
      <StatusBarWrapper backgroundColor="white" barStyle="dark-content" />
    )}
    <View style={overlay ? styles.overlay : styles.white}>
      {children}
    </View>
  </Modal>
);

export const OBDarkModal = ({ children, overlay, transparent, ...props }) => (
  <Modal style={{ flex: 1 }} transparent={overlay || transparent} {...props}>
    {!overlay && (
      <StatusBarWrapper backgroundColor="black" barStyle="light-content" />
    )}
    <View style={overlay ? styles.overlay : styles.black}>
      {children}
    </View>
  </Modal>
);
