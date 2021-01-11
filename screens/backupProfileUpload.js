import React, { PureComponent } from 'react';
import { View, Text, Image, ScrollView } from 'react-native';
import Share from 'react-native-share';

import { footerStyles } from '../utils/styles';
import SMRoundButton from '../components/atoms/SMRoundButton';
import SMTextButton from '../components/atoms/SMTextButton';
import NavBackButton from '../components/atoms/NavBackButton';
import Header from '../components/molecules/Header';

import {I18n} from '../langs/I18n';

const UploadImg = require('../assets/images/upload.png');

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
  },
  resyncContentContainer: {
    flex: 1,
    padding: 16,
    alignItems: 'center',
    minHeight: 500,
  },
  resyncContent: {
    marginTop: 24,
    fontSize: 15,
    color: '#404040',
    lineHeight: 26,
    textAlign: 'center',
  },
  buttonFooter: {
    marginTop: 16,
    flexDirection: 'row',
    justifyContent: 'flex-end',
    alignItems: 'center',
  },
  bold: {
    fontWeight: 'bold',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 20,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 180,
    bottom: 40,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  image: {
    marginTop: 48,
    width: 100,
    height: 100,
  },
};

export default class BackupProfileUpload extends PureComponent {
  handleUpload = async () => {
    const { navigation } = this.props;
    const targetPath = navigation.getParam('targetPath');
    console.warn('-----------', targetPath);

    const shareOptions = {
      message: I18n.t('screens.backupProfileUpload.message'),
      url: `file://${targetPath}`,
    };

    try {
      await Share.open(shareOptions);
    } catch {
    }
  }

  handlePressDone = () => {
    this.props.navigation.pop(3);
  }

  handleGoBack = () => {
    this.props.navigation.goBack();
  }

  render() {
    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <ScrollView contentContainerStyle={styles.resyncContentContainer}>
          <Image style={styles.image} source={UploadImg} />
          <Text style={styles.resyncContent}>
            <Text style={styles.bold}>
              {I18n.t('screens.backupProfileUpload.upload_1')}
            </Text>
            {I18n.t('screens.backupProfileUpload.upload_2')}
          </Text>
          <View style={styles.buttonFooter}>
            <SMRoundButton title= {I18n.t('screens.backupProfileUpload.upload_backup')}  onPress={this.handleUpload} />
          </View>
        </ScrollView>
        <View style={footerStyles.textButtonContainer}>
          <SMTextButton title=  {I18n.t('screens.backupProfileUpload.done')} onPress={this.handlePressDone} />
        </View>
      </View>
    );
  }
}
