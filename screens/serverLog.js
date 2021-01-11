import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, ScrollView } from 'react-native';
import RNFS from 'react-native-fs';
import Share from 'react-native-share';

import Header from '../components/molecules/Header';
import InputGroup from '../components/atoms/InputGroup';
import DescriptionText from '../components/atoms/DescriptionText';
import { screenWrapper } from '../utils/styles';

import Button from '../components/atoms/FullButton';
import NavBackButton from '../components/atoms/NavBackButton';
import { foregroundColor, borderColor, primaryTextColor } from '../components/commonColors';

import {I18n} from '../langs/I18n';

const styles = {
  contentWrapper: {
    flex: 1,
  },
  buttonWrapper: {
    backgroundColor: foregroundColor,
    borderWidth: 1,
    borderColor,
    marginTop: 0,
    marginBottom: 25,
  },
  topButton: {
    marginBottom: 10,
  },
  buttonText: {
    color: primaryTextColor,
  },
};

class ServerLog extends PureComponent {
  handleShareLog = fileName => () => {
    const path = `${RNFS.DocumentDirectoryPath}/Haven/logs/${fileName}.log`;
    const shareOptions = {
      title: 'Email',
      message: 'Here is the log!',
      url: `file://${path}`,
      subject: 'Log',
    };
    Share.open(shareOptions);
  };

  handleGoBack = () => this.props.navigation.goBack();

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.handleGoBack}
        />
        <ScrollView style={styles.contentWrapper} contentContainerStyle={styles.contentWrapper}>
          <InputGroup title="Server Logs" noBorder>
            <DescriptionText>
            Your server logs contain information that can help troubleshoot issues and/or bugs you may be experiencing.
            </DescriptionText>
            <DescriptionText>
            Tapping the buttons below will prompt you to share your logs. Please only share your logs with people you trust. Avoid posting your logs publicly; they contain sensitive information.
            </DescriptionText>
          </InputGroup>
        </ScrollView>
        <Button
          title="Share Server Log"
          wrapperStyle={[styles.buttonWrapper, styles.topButton]}
          textStyle={styles.buttonText}
          onPress={this.handleShareLog('ob')}
          style={{ marginBottom: 10 }}
        />
        <Button
          title="Share IPFS Log"
          textStyle={styles.buttonText}
          wrapperStyle={styles.buttonWrapper}
          onPress={this.handleShareLog('ipfs')}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  settings: state.settings,
});

export default connect(
  mapStateToProps,
)(ServerLog);
