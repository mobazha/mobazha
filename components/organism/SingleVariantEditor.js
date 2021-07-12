import React from 'react';
import deepEqual from 'deep-equal';

import InputGroup from '../atoms/InputGroup';
import TagInput from '../atoms/TagInput';
import TextInput from '../atoms/TextInput';
import TextArea from '../atoms/TextArea';

import {I18n} from '../../langs/I18n';
export default class SingleVariantEditor extends React.Component {
  static getDerivedStateFromProps(props) {
    if (!deepEqual(this.state, props.option)) {
      return { ...props.option };
    } else return {};
  }

  state = {
    name: '',
    description: '',
    variants: [],
  }

  onChangeTitle = (name) => {
    const { index } = this.props;
    this.setState({ name });
    this.props.onChange(index, { ...this.state, name });
  }

  onChangeDescription = (description) => {
    const { index } = this.props;
    this.setState({ description });
    this.props.onChange(index, { ...this.state, description });
  }

  onChangeTags = (variants) => {
    const { index } = this.props;
    this.setState({ variants });
    this.props.onChange(index, { ...this.state, variants });
  }

  removeOption = () => {
    const { index, removeOption } = this.props;
    this.setState({
      name: '',
      description: '',
      variants: [],
    });
    removeOption(index);
  }

  render() {
    const { name, description, variants } = this.state;
    const { index } = this.props;
    return (
      <InputGroup
        title={I18n.t('components.organism.SingleVariantEditor.variant_id', {id: index + 1})}
        actionTitle={I18n.t('components.organism.SingleVariantEditor.delete')}
        actionType="secondary"
        action={this.removeOption}
      >
        <React.Fragment>
          <TextInput
            title={I18n.t('components.organism.SingleVariantEditor.title')}
            value={name}
            required
            onChangeText={this.onChangeTitle}
            placeholder={I18n.t('components.organism.SingleVariantEditor.title_hint')}
            onFocus={this.props.focusInput}
          />
          <TextArea
            title={I18n.t('components.organism.SingleVariantEditor.description')}
            value={description}
            onChangeText={this.onChangeDescription}
            placeholder={I18n.t('components.organism.SingleVariantEditor.description_hint')}
            onFocus={this.props.focusInput}
          />
          <TagInput
            title={I18n.t('components.organism.SingleVariantEditor.choices')}
            required
            noBorder
            initialTags={variants}
            placeholder={I18n.t('components.organism.SingleVariantEditor.choices_hint')}
            onChangeTags={this.onChangeTags}
            onFocus={this.props.focusInput}
          />
        </React.Fragment>
      </InputGroup>
    );
  }
}
