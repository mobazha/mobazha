import React, { PureComponent } from 'react';
import { View, Text, Image, Platform, ScrollView } from 'react-native';
import * as _ from 'lodash';
import DocumentPicker from 'react-native-document-picker';

import Badge from '../components/atoms/Badge';
import SMRoundButton from '../components/atoms/SMRoundButton';
import NavBackButton from '../components/atoms/NavBackButton';
import Header from '../components/molecules/Header';
import { brandColor, bgHightlightColor } from '../components/commonColors';

import {I18n} from '../langs/I18n';

const RestoreImg = require('../assets/images/restore.png');

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
    alignItems: 'center',
  },
  content: {
    flex: 1,
    minHeight: 500,
    alignItems: 'center',
  },
  headerWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
    marginVertical: 8,
  },
  resyncTitle: {
    fontSize: 18,
    color: 'black',
    fontWeight: 'bold',
    lineHeight: 26,
  },
  transparentBetaBadge: {
    opacity: 0,
    marginLeft: 10,
  },
  betaBadge: {
    marginLeft: 10,
  },
  resyncContent: {
    marginTop: 24,
    marginBottom: 16,
    fontSize: 15,
    color: '#404040',
    lineHeight: 26,
  },
  imageContainer: {
    marginTop: 24,
    width: 160,
    height: 160,
    backgroundColor: '#F2FBF3',
    borderRadius: 80,
    justifyContent: 'center',
    alignItems: 'center',
  },
  image: {
    width: 80,
    height: 80,
  },
};

export default class RestoreProfileInit extends PureComponent {
  state = {
    loading: false,
  }

  handleSelectFile = async () => {
    const { navigation } = this.props;
    this.setState({ loading: true });
    try {
      const res = await DocumentPicker.pick({
        type: [Platform.OS === 'ios' ? 'public.data' : 'application/zip'],
      });
      this.setState({ loading: false }, () => {
        navigation.navigate('RestoreProfilePassword', { path: res.uri });
      });
    } catch (err) {
      this.setState({ loading: false });
      if (DocumentPicker.isCancel(err)) {
        // User cancelled the picker, exit any dialogs or menus and move on
      } else {
        throw err;
      }
    }
  }

  handleGoBack = () => this.props.navigation.goBack()

  render() {
    const { loading } = this.state;

    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <ScrollView contentContainerStyle={styles.content}>
          <View style={styles.headerWrapper}>
            <Text style={styles.resyncTitle}>{I18n.t('screens.restoreProfileInit.restore_profile')}</Text>
          </View>
          <View style={styles.imageContainer}>
            <Image style={styles.image} source={RestoreImg} />
          </View>
          <Text style={styles.resyncContent}>
            {I18n.t('screens.restoreProfileInit.restore_hint')}
          </Text>
          <SMRoundButton title={I18n.t('screens.restoreProfileInit.select_file')} onPress={this.handleSelectFile} loading={loading} />
        </ScrollView>
      </View>
    );
  }
}
