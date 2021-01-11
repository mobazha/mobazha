import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback, Platform, Alert } from 'react-native';
import Tags from 'react-native-tags';
import { isEmpty } from 'lodash';

import { formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    width: '100%',
  },
  inputPart: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    minHeight: 62,
    paddingVertical: 15,
  },
  title: {
    width: 150,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderColor: '#e8e8e8',
  },
  containerStyle: { flex: 1 },
  tagInputStyle: {
    backgroundColor: '#FFF',
    paddingLeft: 0,
  },
  tagStyle: {
    marginVertical: 2,
    paddingHorizontal: 11,
    paddingVertical: 7,
    height: 32,
    borderRadius: 2,
    backgroundColor: '#FFF',
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    marginRight: 6,
    marginBottom: 6,
    fontSize: 13,
    fontWeight: 'bold',
    alignItems: 'center',
    justifyContent: 'center',
  },
  tagTextStyle: {
    fontSize: 13,
    fontWeight: 'bold',
    color: '#000',
    padding: 0,
  },
  inputContainerStyle: {
    paddingHorizontal: 3,
    borderRadius: 0,
    borderWidth: 0,
    backgroundColor: 'transparent',
    margin: 0,
    padding: 0,
    height: 29,
  },
  placeholder: {
    fontSize: 15,
    color: Platform.OS === 'ios' ? '#C7C7CD' : '#a8a8a8',
    fontStyle: 'italic',
    textAlign: 'left',
  },
};

export default class TagInput extends PureComponent {
  state = { editing: false, currentValue: '' }

  handleTagPress = (idx) => {
    Alert.alert('Remove choice?', "You can't undo this action.", [
      { text: 'Cancel' },
      {
        text: 'Remove',
        onPress: () => {
          const { initialTags, onChangeTags } = this.props;
          onChangeTags([...initialTags.slice(0, idx), ...initialTags.slice(idx + 1)]);
        },
      },
    ]);
  }

  handleFocus = () => {
    if (this.input) {
      this.input.focus();
    }
  }

  setEditing = () => {
    this.setState({ editing: true }, () => {
      setTimeout(() => {
        this.input.focus();
      }, 200);
    });
  }

  setTextInputRef = (ref) => { this.input = ref; }

  addNewTag = ({ nativeEvent: { text } }) => {
    const { initialTags, onChangeTags } = this.props;
    if (!isEmpty(text.trim())) {
      onChangeTags([...initialTags, text]);
      setTimeout(() => {
        this.input.focus();
      }, 200);
    }
  }

  render() {
    const {
      title,
      noBorder,
      required,
      placeholder,
      initialTags,
      onChangeTags,
      onFocus,
    } = this.props;
    const { editing } = this.state;
    return (
      <TouchableWithoutFeedback onPress={this.handleFocus}>
        <View style={[styles.wrapper, !noBorder && styles.borderBottom]}>
          <View style={styles.inputPart}>
            <Text style={styles.title}>
              {title}
              <Text style={required ? { color: 'red' } : {}}>{required ? '*' : ''}</Text>
            </Text>
            {initialTags.length !== 0 || editing ? (
              <Tags
                initialTags={initialTags}
                onChangeTags={onChangeTags}
                onTagPress={this.handleTagPress}
                createTagOnString={[',']}
                containerStyle={styles.containerStyle}
                tagContainerStyle={styles.tagStyle}
                tagTextStyle={styles.tagTextStyle}
                inputStyle={styles.tagInputStyle}
                inputContainerStyle={styles.inputContainerStyle}
                textInputProps={{
                  ref: this.setTextInputRef,
                  onFocus,
                  onEndEditing: this.addNewTag,
                  placeholder: 'Add tag...',
                }}
                deleteTagOnPress={false}
              />
            ) : (
              <TouchableWithoutFeedback onPress={this.setEditing}>
                <View>
                  <Text style={styles.placeholder}>{placeholder}</Text>
                </View>
              </TouchableWithoutFeedback>
            )}
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}
