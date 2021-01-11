import React from 'react';
import {
  Animated,
  Dimensions,
  FlatList,
  Image,
  Text,
  TouchableWithoutFeedback,
  View,
  ScrollView,
} from 'react-native';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import NavBackButton from '../atoms/NavBackButton';
import NavCloseButton from '../atoms/NavCloseButton';
import DescriptionText from '../atoms/DescriptionText';
import Header from '../molecules/Header';
import { OBLightModal } from '../templates/OBModal';
import { screenWrapper } from '../../utils/styles';
import { foregroundColor, primaryTextColor, brandColor, formLabelColor, borderColor } from '../commonColors';

import BackupImage from '../../assets/images/backup_image.png';
import { getWalletMnemonic } from '../../api/wallet';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('screen');

const styles = {
  container: {
    flex: 1,
    flexDirection: 'row',
    width: SCREEN_WIDTH * 2,
    alignItems: 'center',
    justifyContent: 'center',
  },
  movingScreen: {
    flex: 1,
    width: SCREEN_WIDTH,
    flex: 1,
    backgroundColor: foregroundColor,
  },
  content: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'flex-start',
    paddingHorizontal: 32,
  },
  title: {
    fontSize: 18,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  image: {
    marginVertical: 36,
  },
  buttonWrapper: {
    width: '100%',
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
    marginBottom: ifIphoneX(32, 16),
  },
  button: {
    paddingVertical: 8,
    paddingLeft: 8,
  },
  buttonText: {
    fontSize: 16,
    fontWeight: 'bold',
    color: brandColor,
    textTransform: 'uppercase',
  },
  mnemonic: {
    fontSize: 13,
    lineHeight: 16,
    paddingVertical: 8,
    paddingHorizontal: 12,
    color: primaryTextColor,
    marginVertical: 4,
    marginHorizontal: 4,
    borderWidth: 1,
    borderColor,
    borderRadius: 2,
  },
  mnemonicIndex: {
    color: formLabelColor,
  },
  phraseContainer: {
    maxHeight: 170,
  },
  phraseWrapper: {
    alignItems: 'center',
  },
  description: {
    fontSize: 16,
  },
  noMarginTop: {
    marginTop: 0,
  },
};

const scrollStyleProps = {
  style: {
    flex: 1,
  },
  contentContainerStyle: {
    flex: 1,
  },
};

export default class BackupWallet extends React.Component {
  state = { recoverKeys: [] }
  aniVal = new Animated.Value(0);

  async componentDidUpdate(prevProps) {
    if (!prevProps.show && this.props.show) {
      this.aniVal.setValue(0);
      try {
        const { mnemonic } = await getWalletMnemonic();
        const recoverKeys = mnemonic.split(' ');
        this.setState({ recoverKeys });
      } catch (err) {
        console.log('Failed to fetch Mnemonic Phrase');
      }
    }
  }

  handleBack = () => {
    Animated.timing(this.aniVal, {
      toValue: 0,
      duration: 300,
      useNativeDriver: true,
    }).start();
  }

  handleNext = () => {
    Animated.timing(this.aniVal, {
      toValue: 1,
      duration: 300,
      useNativeDriver: true,
    }).start();
  }

  handleFinish = () => {
    const { onClose, onFinishBackup } = this.props;
    onClose();
    onFinishBackup();
  }

  keyExtractor = item => `key_${item}`;

  renderItem = ({ item, index }) => (
    <Text style={styles.mnemonic}><Text style={styles.mnemonicIndex}>{index + 1}.</Text> {item}</Text>
  )

  render() {
    const { show, onClose } = this.props;
    const { recoverKeys } = this.state;
    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={show}
        onRequestClose={onClose}
      >
        <View style={styles.container}>
          <View style={screenWrapper.wrapper}>
            <Header modal left={<NavCloseButton />} onLeft={onClose} />
            <ScrollView {...scrollStyleProps}>
              <View style={styles.content}>
                <Text style={styles.title}>{I18n.t('components.templates.BackupWallet.backup_wallet')}</Text>
                <Image style={styles.image} source={BackupImage} />
                <DescriptionText style={styles.description}>
                {I18n.t('components.templates.BackupWallet.backup_description1')}
                </DescriptionText>
                <DescriptionText style={styles.description}>
                {I18n.t('components.templates.BackupWallet.backup_description2')}                  
                </DescriptionText>
                <View style={{ flex: 1 }} />
                <View style={styles.buttonWrapper}>
                  <TouchableWithoutFeedback onPress={this.handleNext}>
                    <View style={styles.button}>
                      <Text style={styles.buttonText}>{I18n.t('components.templates.BackupWallet.next')}</Text>
                    </View>
                  </TouchableWithoutFeedback>
                </View>
              </View>
            </ScrollView>
          </View>
          <Animated.View
            style={[
              styles.movingScreen,
              {
                transform: [{
                  translateX: this.aniVal.interpolate({
                    inputRange: [0, 1],
                    outputRange: [0, -SCREEN_WIDTH],
                  }),
                }],
              },
            ]}
          >
            <Header modal left={<NavBackButton />} onLeft={this.handleBack} />
            <ScrollView {...scrollStyleProps}>
              <View style={styles.content}>
                <Text style={styles.title}>{I18n.t('components.templates.BackupWallet.recovery_phrase')}</Text>
                <DescriptionText style={[styles.description, styles.noMarginTop]}>
                {I18n.t('components.templates.BackupWallet.phrase_hint')}
                </DescriptionText>
                <FlatList
                  numColumns={3}
                  style={styles.phraseContainer}
                  contentContainerStyle={styles.phraseWrapper}
                  data={recoverKeys}
                  keyExtractor={this.keyExtractor}
                  renderItem={this.renderItem}
                />
                <DescriptionText style={styles.description}>
                {I18n.t('components.templates.BackupWallet.writedown_hint')}                  
                </DescriptionText>
                <View style={{ flex: 1 }} />
                <View style={styles.buttonWrapper}>
                  <TouchableWithoutFeedback onPress={this.handleFinish}>
                    <View style={styles.button}>
                      <Text style={styles.buttonText}>{I18n.t('components.templates.BackupWallet.done')}</Text>
                    </View>
                  </TouchableWithoutFeedback>
                </View>
              </View>
            </ScrollView>
          </Animated.View>
        </View>
      </OBLightModal>
    );
  }
}

