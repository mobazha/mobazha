import React from 'react';
import { Alert, Animated, View, Dimensions } from 'react-native';

import NavCloseButton from '../atoms/NavCloseButton';
import NavBackButton from '../atoms/NavBackButton';
import LinkText from '../atoms/LinkText';
import TextArea from '../atoms/TextArea';
import InputGroup from '../atoms/InputGroup';
import Header from '../molecules/Header';
import RadioFilter from '../molecules/RadioFilter';
import { OBLightModal } from './OBModal';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get('screen');

const reportProfileOptions = [
  { value: 'Inappropriate content', label: 'Inappropriate content' },
  { value: 'Harrasment / Abuse', label: 'Harrasment / Abuse' },
  { value: 'Impersonation', label: 'Impersonation' },
  { value: 'Spam or Suspicious Content', label: 'Spam or Suspicious Content' },
  { value: 'Other', label: 'Other' },
];

const reportListingOptions = [
  { value: 'Offensive content', label: 'Offensive content' },
  { value: 'Inaccurate or fraudulent', label: 'Inaccurate or fraudulent' },
  { value: 'Illegal in my area', label: 'Illegal in my area' },
  { value: 'Other', label: 'Other' },
];

const styles = {
  wrapper: {
    width: SCREEN_WIDTH * 2,
    height: SCREEN_HEIGHT,
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  container: {
    width: SCREEN_WIDTH,
    height: SCREEN_HEIGHT,
  },
};

export default class ReportTemplate extends React.Component {
  state = { option: '', reason: '', showModal: false };

  onChangeOption = (option) => { this.setState({ option }); };

  onChangeReason = (reason) => { this.setState({ reason }); };

  setInputRef = (ref) => { this.input = ref; }

  aniVal = new Animated.Value(0);

  submit = () => {
    const { reason, option } = this.state;
    this.setState({ showModal: false }, () => {
      this.props.submit(reason, option);
    });
  }

  startStep() {
    this.aniVal = new Animated.Value(0);
    this.setState({ option: '', reason: '', showModal: true });
  }

  animate = (toValue) => {
    if (toValue === 0 && this.input) {
      this.input.blur();
    }
    Animated.timing(this.aniVal, { toValue, duration: 300 }).start(() => {
      if (toValue === 1 && this.input) {
        this.input.focus();
      }
    });
  }

  nextStep = () => {
    const { option } = this.state;
    if (option !== '') {
      this.animate(1);
    } else {
      Alert.alert('Ooops!', 'Please enter a reason for reporting this content.');
    }
  }

  prevStep = () => {
    this.setState({ reason: '' }, () => {
      this.animate(0);
    });
  }

  cancel = () => {
    this.setState({ showModal: false, option: '' });
  }

  render() {
    const {
      showModal, option, reason,
    } = this.state;
    const { type = 'profile' } = this.props;
    const title = type === 'profile' ? 'Why are you reporting this profile?' : 'Why are you reporting this?';
    const reportingOptions = type === 'profile' ? reportProfileOptions : reportListingOptions;
    return (
      <OBLightModal
        animationType="slide"
        visible={showModal}
        onRequestClose={this.cancel}
      >
        <Animated.View
          style={[
            styles.wrapper,
            {
              marginLeft: this.aniVal.interpolate({
                inputRange: [0, 1],
                outputRange: [0, -SCREEN_WIDTH],
              }),
            },
          ]}
        >
          <View style={styles.container}>
            <Header
              modal
              left={<NavCloseButton />}
              onLeft={this.cancel}
              right={<LinkText text="Next" />}
              onRight={this.nextStep}
            />
            <RadioFilter
              wrapperStyle={{ flex: 1 }}
              style={{ flex: 1 }}
              secondary
              title={title}
              selected={option}
              options={reportingOptions}
              onChange={this.onChangeOption}
            />
          </View>
          <View style={styles.container}>
            <Header
              modal
              left={<NavBackButton />}
              onLeft={this.prevStep}
              right={<LinkText text="Submit" />}
              onRight={this.submit}
            />
            <InputGroup title="Please describe the issue (optional)" noBorder>
              <TextArea
                ref={this.setInputRef}
                noBorder
                placeholder="Provide as much details as possible"
                value={reason}
                onChangeText={this.onChangeReason}
              />
            </InputGroup>
          </View>
        </Animated.View>
      </OBLightModal>
    );
  }
}
