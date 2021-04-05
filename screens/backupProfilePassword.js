import React, { PureComponent } from 'react';
import { View, Text, KeyboardAvoidingView, Alert, ScrollView } from 'react-native';
import * as _ from 'lodash';
import RnFs from 'react-native-fs';
import { zip, zipWithPassword, subscribe } from 'react-native-zip-archive';
import moment from 'moment';
import { connect } from 'react-redux';

import NavBackButton from '../components/atoms/NavBackButton';
import InputGroup from '../components/atoms/InputGroup';
import TextInput from '../components/atoms/TextInput';
import Header from '../components/molecules/Header';
import SMTextButton from '../components/atoms/SMTextButton';
import { footerStyles } from '../utils/styles';
import { SERVER_PATH } from '../utils/server';
import { keyboardAvoidingViewSharedProps } from '../utils/keyboard';
import { purgeCache } from '../api/cache';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
  },
  resyncContentContainer: {
    flex: 1,
    padding: 16,
  },
  resyncTitle: {
    fontSize: 15,
    color: 'black',
    fontWeight: 'bold',
    lineHeight: 26,
  },
  resyncContent: {
    marginTop: 16,
    fontSize: 15,
    color: '#404040',
    lineHeight: 26,
  },
  buttonFooter: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    alignItems: 'center',
  },
  bold: {
    fontWeight: 'bold',
  },
};

class BackupProfilePassword extends PureComponent {
  state = {
    password: '',
    cpassword: '',
    loadingText: null,
  }

  handleBackup = async () => {
    const { peerID } = this.props;
    if (!peerID) {
      return;
    }

    const { password, cpassword } = this.state;
    if (_.isEmpty(password)) {
      Alert.alert(I18n.t('screens.backupProfilePassword.password_empty'), I18n.t('screens.backupProfilePassword.password_empty_hint'));
      return;
    }
    if (password !== cpassword) {
      Alert.alert(I18n.t('screens.backupProfilePassword.password_mismatch'), I18n.t('screens.backupProfilePassword.password_mismatch_hint')) ;
      return;
    }

    this.setState({ loadingText: I18n.t('screens.backupProfilePassword.take_a_minute')});

    try {
      this.targetPath = `${RnFs.DocumentDirectoryPath}/havenBackup_${peerID.substring(0, 6)}_${moment().format('YYYYMMDDhhmmss')}.zip`;

      const targetExists = await RnFs.exists(this.targetPath);
      if (targetExists) {
        await RnFs.unlink(this.targetPath);
      }

      await purgeCache();

      const result = zipWithPassword(SERVER_PATH, this.targetPath, password, 'AES-256');
      subscribe(this.handleZipEvent);
      console.warn(I18n.t('screens.backupProfilePassword.backup_done'), result);
    } catch (err) {
      console.warn(I18n.t('screens.backupProfilePassword.backup_failed'), err);
      this.setState({ loadingText: null });
    }
  }

  handleZipEvent = (event) => {
    if (event.progress === 1) {
      this.setState({ loadingText: null }, () => {
        this.props.navigation.navigate('BackupProfileUpload', { targetPath: this.targetPath });
      });
    }
  }

  handleGoBack = () => {
    this.props.navigation.goBack();
  }

  handlePasswordUpdate = field => (value) => {
    this.setState({ [field]: value });
  }

  render() {
    const { loadingText, password, cpassword } = this.state;
    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <KeyboardAvoidingView style={{ flex: 1 }} {...keyboardAvoidingViewSharedProps}>
          <ScrollView style={{ flex: 1 }} contentContainerStyle={styles.resyncContentContainer}>
            <InputGroup title={I18n.t('screens.backupProfilePassword.backupProfileUpload')} noPadding noHeaderPadding>
              <TextInput
                title={I18n.t('screens.backupProfilePassword.set_password')}
                value={password}
                placeholder={I18n.t('screens.backupProfilePassword.password')}
                onChangeText={this.handlePasswordUpdate('password')}
                required
                secureTextEntry
                autoFocus
              />
              <TextInput
                title={I18n.t('screens.backupProfilePassword.confirm')}
                value={cpassword}
                placeholder={I18n.t('screens.backupProfilePassword.confirm_password')}
                onChangeText={this.handlePasswordUpdate('cpassword')}
                required
                noBorder
                secureTextEntry
              />
            </InputGroup>
            <Text style={styles.resyncContent}>
            {I18n.t('screens.backupProfilePassword.hint1')} <Text style={styles.bold}>{I18n.t('screens.backupProfilePassword.hint2')}</Text>
              {I18n.t('screens.backupProfilePassword.hint3')}
            </Text>
          </ScrollView>
          <View style={footerStyles.textButtonContainer}>
            <SMTextButton title={I18n.t('screens.backupProfilePassword.next')} onPress={this.handleBackup} loadingText={loadingText} />
          </View>
        </KeyboardAvoidingView>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  peerID: _.get(state, 'profile.data.peerID'),
});

export default connect(mapStateToProps)(BackupProfilePassword);
