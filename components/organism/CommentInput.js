import React from 'react';
import { connect } from 'react-redux';
import { View, TouchableOpacity, Keyboard } from 'react-native';
import { get } from 'lodash';
import { AutoGrowingTextInput } from 'react-native-autogrow-textinput';

import AvatarImage from '../atoms/AvatarImage';
import LinkText from '../atoms/LinkText';
import { foregroundColor, primaryTextColor, borderColor, formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    width: '100%',
    bottom: 0,
    borderTopWidth: 1,
    borderTopColor: borderColor,
  },
  inputWrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    paddingHorizontal: 15,
    paddingVertical: 12,
  },
  input: {
    flex: 1,
    marginHorizontal: 10,
    paddingTop: 13,
    paddingBottom: 13,
    fontSize: 15,
    fontWeight: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  inputAvatarImage: { width: 40 },
};

class Comments extends React.PureComponent {
  state = {
    comment: '',
  };

  setInputRef = (ref) => { this.input = ref; }

  setFocus = () => { if (this.input) { this.input.focus(); } }

  handleChangeComment = (comment) => {
    this.setState({ comment });
  }

  submitComment = () => {
    const { comment } = this.state;
    this.props.submitComment(comment);
    this.setState({ comment: '' });
    if (this.input) {
      this.input.blur();
    }
    Keyboard.dismiss();
  }

  render() {
    const { profile } = this.props;
    const thumbnail = get(profile, 'avatarHashes.small');
    const { comment } = this.state;
    return (
      <View style={styles.wrapper}>
        <View style={styles.inputWrapper}>
          <AvatarImage thumbnail={thumbnail} style={styles.inputAvatarImage} />
          <AutoGrowingTextInput
            style={styles.input}
            onChangeText={this.handleChangeComment}
            value={comment}
            placeholder="Write a comment"
            placeholderTextColor={formLabelColor}
            ref={this.setInputRef}
          />
          {comment !== '' && (
            <TouchableOpacity onPress={this.submitComment}>
              <LinkText text="Post" />
            </TouchableOpacity>
          )}
        </View>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
});

export default connect(mapStateToProps, null, null, { withRef: true })(Comments);
