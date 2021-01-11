import React from 'react';
import { Button, Modal, Text, View } from 'react-native';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'rgba(255, 255, 255, 0.9)',
    padding: 50,
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    flexGrow: 1,
  },
  title: {
    fontWeight: 'bold',
    textAlign: 'center',
    marginBottom: 10,
  },
  body: {
    textAlign: 'center',
  },
};

export default function ErrorModal(props) {
  const { title, error, onPress, buttonText } = props;
  return (
    <Modal transparent >
      <View style={styles.wrapper}>
        <Text style={styles.title}>{title}</Text>
        <Text style={styles.body}>{I18n.t('components.organism.ErrorModal.error_message')}</Text>
        <Button title={buttonText} onPress={onPress} />
      </View>
    </Modal>);
}
