import React from 'react';
import { Text } from 'react-native';

import InputGroup from '../atoms/InputGroup';
import OptionGroup from '../atoms/OptionGroup';
import FormLabelText from '../atoms/FormLabelText';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const style = {
  fontSize: 15,
  fontWeight: 'normal',
  fontStyle: 'normal',
  letterSpacing: 0,
  textAlign: 'left',
  color: primaryTextColor,
  paddingVertical: 20,
};

export default class TagEditor extends React.PureComponent {
  render() {
    const { count } = this.props;
    return (
      <InputGroup title={I18n.t('components.organism.TagEditor.tags')} showPencil onPress={this.props.onPress}>
        <OptionGroup noBorder noArrow>
          { count > 0 ?
            <Text style={style}>{I18n.t('components.organism.TagEditor.tags_info', {count:count})}</Text>
          :
            <FormLabelText text={I18n.t('components.organism.TagEditor.add_hint')} />
          }
        </OptionGroup>
      </InputGroup>
    );
  }
}
