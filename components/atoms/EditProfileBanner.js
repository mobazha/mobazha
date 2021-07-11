import React, { PureComponent } from 'react';
import { View } from 'react-native';

import SaveButton from './FullButton';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'white',
    flexDirection: 'row',
    alignItems: 'center',
    borderTopWidth: 1,
    borderColor: '#e8e8e8',
  },
  leftSpacer: {
    flex: 5,
    alignSelf: 'center',
    paddingLeft: 16,
  },
  saveButtonContainer: {
    flex: 3,
  },
  saveButton: {
    backgroundColor: '#00bf65',
    marginLeft: 0,
  },
};

class EditProfileBanner extends PureComponent {
  render() {
    const { onSave } = this.props;
    return (
      <View style={styles.wrapper}>
        <View style={styles.leftSpacer} />
        <View style={styles.saveButtonContainer}>
          <SaveButton
            wrapperStyle={styles.saveButton}
            title={I18n.t('components.atoms.EditProfileBanner.save')}
            onPress={onSave}
          />
        </View>
      </View>
    );
  }
}

export default EditProfileBanner;
