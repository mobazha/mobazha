import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import {
  View,
  ScrollView,
  Alert,
  KeyboardAvoidingView,
  Keyboard,
  InteractionManager,
  TouchableWithoutFeedback,
} from 'react-native';
import Tags from 'react-native-tags';
import { filter, findIndex, isEmpty } from 'lodash';

import Header from '../components/molecules/Header';
import TagSuggestion from '../components/organism/TagSuggestion';
import { greenColor } from '../components/commonColors';
import LinkText from '../components/atoms/LinkText';
import NavBackButton from '../components/atoms/NavBackButton';

import { setBasicInfo } from '../reducers/createListing';
import { setTags } from '../reducers/appstate';
import { eventTracker } from '../utils/EventTracker';
import { keyboardAvoidingViewSharedProps } from '../utils/keyboard';

import {I18n} from '../langs/I18n';

const styles = {
  content: {
    flex: 1,
    paddingTop: 20,
    paddingHorizontal: 14,
  },
  wrapper: {
    flex: 1,
    backgroundColor: '#FFF',
  },
  tagInputStyle: {
    backgroundColor: '#FFF',
    paddingLeft: 0,
  },
  tagStyle: {
    marginVertical: 2,
    paddingHorizontal: 11,
    paddingVertical: 6,
    height: 32,
    borderRadius: 2,
    backgroundColor: '#FFF',
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    marginRight: 6,
    marginBottom: 6,
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
  },
};

const parseTag = (tag) => {
  const parts = tag.split('#');
  if (parts.length > 1) {
    return tag;
  } else {
    return `#${tag}`;
  }
};

const originTag = (tag) => {
  const parts = tag.split('#');
  if (parts.length > 1) {
    return parts[1];
  } else {
    return tag;
  }
};

class TagInput extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      tags: props.tags.map(tag => `#${tag}`),
      showSuggestion: false,
    };
  }

  componentDidMount() {
    InteractionManager.addListener('interactionStart', () => {
      Keyboard.dismiss();
    });
    this.focusListener = this.props.navigation.addListener('didFocus', this.setInputFocus);
  }

  componentWillUnmount() {
    this.focusListener.remove();
  }

  onChangeTags = (tags) => {
    const newTags = tags.map(tag => parseTag(tag));
    this.setState({
      tags: newTags,
    });
  };

  onTagPress = (index) => {
    Alert.alert(I18n.t('screens.tagEditor.remove_tag'), I18n.t('screens.tagEditor.cannot_undo'), [
      { text: I18n.t('screens.tagEditor.cancel') },
      {
        text: I18n.t('screens.tagEditor.remove'),
        onPress: () => {
          this.removeTag(index);
        },
      },
    ], { cancelable: false });
  };

  onSelectSuggestion = (tag) => {
    const { tags } = this.state;
    this.setState({
      tags: [...tags, `#${tag}`],
    });
  };

  onInputFocus = () => {
    this.setState({
      showSuggestion: true,
    });
  };

  onEndEditing = ({ nativeEvent: { text } }) => {
    const { tags } = this.state;
    if (!isEmpty(text.trim())) {
      this.setState({
        tags: [...tags, parseTag(text.trim())],
      });
      this.tagInput.clear();
      setTimeout(this.setInputFocus, 500);
    }
  };

  getTagSuggestions = () => {
    const { recentTags } = this.props;
    const { tags } = this.state;
    const orgTags = tags.map(tag => originTag(tag));
    const suggestion = filter(
      recentTags,
      recentTag => findIndex(orgTags, orgTag => orgTag === recentTag) === -1,
    );
    return suggestion;
  };

  setInputRef = (ref) => {
    this.tagInput = ref;
  };

  setInputFocus = () => {
    if (this.tagInput) {
      this.tagInput.focus();
    }
  }

  goBack = () => {
    Keyboard.dismiss();
    Alert.alert(I18n.t('screens.tagEditor.are_you_sure'), I18n.t('screens.tagEditor.unsaved_discard'), [
      { text: I18n.t('screens.tagEditor.cancel') },
      {
        text: I18n.t('screens.tagEditor.ok'),
        onPress: () => {
          this.props.navigation.goBack();
        },
      }
    ], { cancelable: false });
  };

  save = () => {
    const { tags } = this.state;
    const orgTags = tags.map(tag => originTag(tag));
    Keyboard.dismiss();
    this.props.setBasicInfo({ tags: orgTags });
    this.props.setTags(orgTags);
    eventTracker.trackEvent('CreateListing-AddedTags');

    this.props.navigation.goBack();
  };

  removeTag = (idx) => {
    const { tags } = this.state;
    const newTags = [...tags.slice(0, idx), ...tags.slice(idx + 1)];
    this.setState({
      tags: newTags,
    });
  };

  render() {
    const { tags, showSuggestion } = this.state;
    const suggestions = this.getTagSuggestions();
    return (
      <KeyboardAvoidingView style={styles.wrapper} {...keyboardAvoidingViewSharedProps}>
        <Header
          title={I18n.t('screens.tagEditor.tags')}
          left={<NavBackButton />}
          onLeft={this.goBack}
          right={<LinkText text={I18n.t('screens.tagEditor.done')} color={greenColor} />}
          onRight={this.save}
        />
        <ScrollView style={{ flex: 1 }} contentContainerStyle={{ flexGrow: 1 }}>
          <TouchableWithoutFeedback onPress={this.setInputFocus}>
            <View style={styles.content}>
              <Tags
                initialTags={tags}
                onChangeTags={this.onChangeTags}
                onTagPress={this.onTagPress}
                createTagOnString={[',', '.']}
                tagContainerStyle={styles.tagStyle}
                tagTextStyle={styles.tagTextStyle}
                inputStyle={styles.tagInputStyle}
                inputContainerStyle={styles.inputContainerStyle}
                textInputProps={{
                  ref: this.setInputRef,
                  onFocus: this.onInputFocus,
                  onEndEditing: this.onEndEditing,
                }}
                deleteOnTagPress={false}
              />
            </View>
          </TouchableWithoutFeedback>
        </ScrollView>
        {showSuggestion && (
          <TagSuggestion
            title={I18n.t('screens.tagEditor.recent')}
            suggestions={suggestions}
            onSelect={this.onSelectSuggestion}
          />
        )}
      </KeyboardAvoidingView>
    );
  }
}

const mapStateToProps = ({ createListing: { tags }, appstate: { recent_tags } }) => ({
  tags,
  recentTags: recent_tags,
});
const mapDispatchToProps = {
  setBasicInfo,
  setTags,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(TagInput);
