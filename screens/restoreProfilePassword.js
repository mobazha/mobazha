import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import {
  View, Text, ScrollView, DeviceEventEmitter, NativeEventEmitter, Platform, NativeModules, Alert, KeyboardAvoidingView,
} from 'react-native';
import RnFs from 'react-native-fs';
import { unzipWithPassword } from 'react-native-zip-archive';
import RNRestart from 'react-native-restart';
import SplashScreen from 'react-native-splash-screen';

import { initializeLogin } from '../reducers/appstate';
import NavBackButton from '../components/atoms/NavBackButton';
import TextInput from '../components/atoms/TextInput';
import Header from '../components/molecules/Header';
import ServerBridgeNativeModule from '../ServerBridgeNativeModule';
import { primaryTextColor } from '../components/commonColors';
import SMTextButton from '../components/atoms/SMTextButton';
import { footerStyles } from '../utils/styles';
import { SERVER_PATH, SERVER_UNZIP_PATH } from '../utils/server';
import { keyboardAvoidingViewSharedProps } from '../utils/keyboard';
import {I18n} from '../langs/I18n'


const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
  },
  resyncContentContainer: {
    flex: 1,
    padding: 16,
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
  title: {
    fontSize: 16,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: -0.1,
    textAlign: 'left',
    color: primaryTextColor,
  },
};

const { ModuleWithEmitter } = NativeModules;
const iOSeventEmitter = new NativeEventEmitter(ModuleWithEmitter);

class RestoreProfilePassword extends PureComponent {
  state = {
    loadingText: null,
  }

  UNSAFE_componentWillMount() {
    if (Platform.OS === 'ios') {
      iOSeventEmitter.addListener('onServerStopped', this.handleServerStopped);
      iOSeventEmitter.addListener('onServerStopFailed', this.handleServerStopFailed);
      iOSeventEmitter.addListener('onServerStarted', this.handleServerStarted);
      iOSeventEmitter.addListener('onServerStartFailed', this.handleServerStartFailed);
    } else {
      DeviceEventEmitter.addListener('onServerStopped', this.handleServerStopped);
      DeviceEventEmitter.addListener('onServerStopFailed', this.handleServerStopFailed);
      DeviceEventEmitter.addListener('onServerStarted', this.handleServerStarted);
      DeviceEventEmitter.addListener('onServerStartFailed', this.handleServerStartFailed);

      DeviceEventEmitter.addListener('onZipFilePathFetched', this.handleZipFilePathFetched);
      DeviceEventEmitter.addListener('onZipFilePathFetchFailed', this.handleZipFilePathFetchFailed);
    }
  }

  handleRestore = async () => {
    this.setState({ loadingText: I18n.t('screens.restoreProfilePassword.loading_hint') });

    const { navigation } = this.props;
    let zipPath = navigation.getParam('path');
    console.warn('zip path from picker', zipPath);

    if (Platform.OS === 'ios') {
      zipPath = `/${navigation.getParam('path').replace(/^file:\/\/\//g, '')}`;
      await this.handleZipFilePathFetched(zipPath);
    } else {
      if (zipPath.includes('content://')) {
        ServerBridgeNativeModule.downloadFromCloudContentUri(zipPath);
      } else {
        zipPath = `/${navigation.getParam('path').replace(/^file:\/\/\//g, '')}`;
        await this.handleZipFilePathFetched(zipPath);
      }
    }
  }

  handleZipFilePathFetched = async (rawZipPath) => {
    const { password } = this.state;
    const zipPath = decodeURI(rawZipPath);
    console.warn('zip path fetched (from content uri)', zipPath);
    try {
      await RnFs.unlink(SERVER_UNZIP_PATH);
    } catch (_) {} // eslint-disable-line no-empty

    try {
      const unzippedPath = await unzipWithPassword(zipPath, SERVER_UNZIP_PATH, password);
      console.warn('Succeed to unzip backup file', unzippedPath);
    } catch (err) {
      this.handleUnzipFailed(I18n.t('screens.restoreProfilePassword.wrong_password'));
      return;
    }

    ServerBridgeNativeModule.stop();
  }

  handleZipFilePathFetchFailed = () => {
    this.handleUnzipFailed(I18n.t('screens.restoreProfilePassword.failed_download'));
  }

  handleServerStopped = async () => {
    console.warn('server stopped');
    const tempPath = `${RnFs.DocumentDirectoryPath}/Mobazha_`;

    try {
      await RnFs.moveFile(SERVER_PATH, tempPath);
      await RnFs.moveFile(SERVER_UNZIP_PATH, SERVER_PATH);
      this.tempPath = tempPath;
      console.warn('Succeed tempPath backup');
    } catch (err) {
      console.warn('Failed tempPath backup', err);
    }

    ServerBridgeNativeModule.start();
  }

  handleServerStopFailed = () => {
    console.warn('server stop failed');
    this.setState({ loadingText: null });
  }

  handleServerStarted = async () => {
    console.warn('new server started');

    try {
      if (this.tempPath) {
        RnFs.unlink(this.tempPath);
      }
    } catch (err) {
      console.warn('Failed to remove old server folder', err);
    }

    SplashScreen.show();
    RNRestart.Restart();
  }

  handleServerStartFailed = async () => {
    console.warn('Failed to rstart server after unzip');
    this.setState({ loadingText: null }, async () => {
      if (this.tempPath) {
        try {
          await RnFs.moveFile(this.tempPath, SERVER_PATH);
          this.tempPath = null;
          ServerBridgeNativeModule.start();
        } catch (err) {
          // eslint-disable-next-line no-empty
        }
      }
    });
  }

  handleUnzipFailed = async (errMsg) => {
    this.setState({ loadingText: null }, () => {
      Alert.alert(I18n.t('screens.restoreProfilePassword.Ooops'), errMsg);
    });
  }

  handleGoBack = () => {
    this.props.navigation.goBack();
  }

  handlePasswordChange = (value) => {
    this.setState({ password: value });
  }

  render() {
    const { loadingText, password } = this.state;
    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <KeyboardAvoidingView style={{ flex: 1 }} {...keyboardAvoidingViewSharedProps}>
          <ScrollView style={{ flex: 1 }} contentContainerStyle={styles.resyncContentContainer}>
            <Text style={styles.title}>{I18n.t('screens.restoreProfilePassword.enter_password')}</Text>
            <TextInput
              title={I18n.t('screens.restoreProfilePassword.password')}
              value={password}
              onChangeText={this.handlePasswordChange}
              required
              placeholder={I18n.t('screens.restoreProfilePassword.enter_password')}
              autoFocus
              secureTextEntry
            />
            <Text style={styles.resyncContent}>
            {I18n.t('screens.restoreProfilePassword.enter_password_hint')}
            </Text>
          </ScrollView>
          <View style={footerStyles.textButtonContainer}>
            <SMTextButton
              title={I18n.t('screens.restoreProfilePassword.restore')}
              onPress={this.handleRestore}
              loadingText={loadingText}
            />
          </View>
        </KeyboardAvoidingView>
      </View>
    );
  }
}

const mapDispatchToProps = { initializeLogin };

export default connect(
  null,
  mapDispatchToProps,
)(RestoreProfilePassword);
