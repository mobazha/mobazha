import React from 'react';
import { View, Alert, Platform, ScrollView, KeyboardAvoidingView, Dimensions, Keyboard } from 'react-native';
import { AutoGrowingTextInput } from 'react-native-autogrow-textinput';
import { connect } from 'react-redux';
import { get } from 'lodash';

import { OBLightModal } from '../OBModal';
import PostButton from '../../atoms/PostButton';
import NavCloseButton from '../../atoms/NavCloseButton';
import AvatarImage from '../../atoms/AvatarImage';
import Header from '../../molecules/Header';

import { primaryTextColor, secondaryTextColor } from '../../commonColors';

import { keyboardAvoidingViewSharedProps } from '../../../utils/keyboard';

import {I18n} from '../../../langs/I18n';

const { height } = Dimensions.get('screen');

const BUTTON_WRAPPER_PADDING = Platform.OS === 'ios' ? 15 : 10;
const INPUT_WRAPPER_BOTTOM_MARGIN = Platform.OS === 'ios' && height > 800 ? 60 : 0;

const styles = {
  inputContainer: {
    flex: 1,
    paddingHorizontal: 16,
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  avatarImage: {
    width: 38,
    height: 38,
    marginRight: 13,
  },
  inputWrapper: {
    flex: 1,
    marginTop: 6,
  },
  input: {
    fontSize: 19,
    marginTop: Platform.OS === 'android' ? -8 : 0,
    width: '100%',
    color: primaryTextColor,
    marginBottom: 20,
  },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
    paddingHorizontal: 15,
    paddingVertical: BUTTON_WRAPPER_PADDING,
  },
};

class SocialPostBase extends React.Component {
  state = {
    keyboardShown: false,
  };

  componentDidMount() {
    Keyboard.addListener('keyboardWillShow', () => this.setState({ keyboardShown: true }));
    Keyboard.addListener('keyboardDidHide', () => this.setState({ keyboardShown: false }));
  }
  setInputRef
  render() {
    const {
      profile, onPost, comment, showModal, onChangeComment, onHideModal, setInputRef, children,
    } = this.props;
    const { keyboardShown } = this.state;
    const thumbnail = get(profile, 'avatarHashes.tiny');
    return (
      <OBLightModal
        animationType="slide"
        transparent
        visible={showModal}
        onRequestClose={onHideModal}
      >
        <KeyboardAvoidingView style={{ flex: 1 }} {...keyboardAvoidingViewSharedProps}>
          <Header left={<NavCloseButton />} onLeft={onHideModal} modal />
          <View style={styles.inputContainer}>
            <AvatarImage style={styles.avatarImage} thumbnail={thumbnail} />
            <ScrollView style={styles.inputWrapper}>
              <AutoGrowingTextInput
                style={styles.input}
                value={comment}
                onChangeText={onChangeComment}
                placeholder="Add Comment"
                placeholderTextColor={secondaryTextColor}
                ref={setInputRef}
              />
              {children}
            </ScrollView>
          </View>
          <View
            style={[styles.footer, keyboardShown && { marginBottom: INPUT_WRAPPER_BOTTOM_MARGIN }]}
          >
            <PostButton disabled={comment === ''} onPress={onPost} />
          </View>
        </KeyboardAvoidingView>
      </OBLightModal>
    );
  }
}

const mapStateToProps = state => ({ profile: state.profile.data || {} });

export default connect(mapStateToProps)(SocialPostBase);
