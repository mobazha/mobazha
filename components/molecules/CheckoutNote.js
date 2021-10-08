import React from 'react';
import { View, Text } from 'react-native';
import * as _ from 'lodash';
import { connect } from 'react-redux';

import InputGroup from '../atoms/InputGroup';
import { primaryTextColor, foregroundColor, borderColor, linkTextColor, formLabelColor } from '../commonColors';

import { EditIcon } from '../../utils/checkout';
import { OBLightModal } from '../templates/OBModal';
import AvatarImage from '../atoms/AvatarImage';
import { chatStyles } from '../../utils/styles';
import PlaceholderStyleTextInput from '../atoms/PlaceholderStyleTextInput';
import Header from './Header';
import NavCloseButton from '../atoms/NavCloseButton';
import LinkText from '../atoms/LinkText';
import { setCheckoutNote } from '../../reducers/appstate';

import {I18n} from '../../langs/I18n';

const MAJOR_PADDING = chatStyles.avatarImage.marginLeft;

const styles = {
  wrapper: {
    paddingTop: 12,
    paddingBottom: 16,
  },
  contentContainer: {
    flex: 1,
    backgroundColor: foregroundColor,
    flexDirection: 'row',
    borderColor,
    borderBottomWidth: 1,
    paddingTop: 7,
  },
  input: {
    marginLeft: MAJOR_PADDING,
    flex: 1,
    width: '100%',
    fontSize: 19,
    fontWeight: '400',
    fontStyle: 'normal',
    color: primaryTextColor,
    backgroundColor: 'white',
    textAlignVertical: 'top',
    paddingTop: 18,
    paddingRight: MAJOR_PADDING,
    lineHeight: 20,
  },
  placeholder: {
    fontSize: 19,
    paddingTop: 18,
    lineHeight: 20,
  },
  text: {
    backgroundColor: 'white',
    padding: MAJOR_PADDING,
    textAlign: 'right',
    color: 'rgb(200, 199, 204)',
  },
  note: {
    color: 'black',
    fontSize: 15,
  },
  noteTip: {
    color: formLabelColor,
    fontSize: 14,
    fontStyle: 'italic',
  },
};

class CheckoutNote extends React.PureComponent {
  state = {
    showModal: false,
    note: '',
  }

  handleOpenModal = () => {
    const { checkoutNote } = this.props;
    this.setState({ showModal: true, note: checkoutNote });
  }

  handleHideModal = () => {
    this.setState({ showModal: false });
  }

  handleSaveNote = () => {
    const { setCheckoutNote } = this.props;
    const { note } = this.state;
    setCheckoutNote(note);
    this.handleHideModal();
  }

  render() {
    const { profile, checkoutNote } = this.props;
    const { note, showModal } = this.state;
    return (
      <InputGroup
        contentStyle={styles.wrapper}
        title={I18n.t('components.molecules.CheckoutNote.note')}
        action={this.handleOpenModal}
        actionTitle={EditIcon}
        noBorder
      >
        {_.isEmpty(checkoutNote) ? (
          <Text style={styles.noteTip}>{I18n.t('components.molecules.CheckoutNote.add_note')}</Text>
        ) : (
          <Text style={styles.note}>{checkoutNote}</Text>
        )}
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={this.handleHideModal}
        >
          <Header
            modal
            left={<NavCloseButton />}
            onLeft={this.handleHideModal}
            right={<LinkText text="Save" color={linkTextColor} />}
            onRight={this.handleSaveNote}
          />
          <View style={styles.contentContainer}>
            <AvatarImage
              style={chatStyles.avatarImage}
              thumbnail={_.get(profile, 'avatarHashes.tiny')}
            />
            <PlaceholderStyleTextInput
              placeholder={I18n.t('components.molecules.CheckoutNote.add_seller_note')}
              style={styles.input}
              onChangeText={note => this.setState({ note })}
              value={note}
              underlineColorAndroid="transparent"
              multiline
              autoFocus
              placeholderStyle={styles.placeholder}
            />
          </View>
        </OBLightModal>
      </InputGroup>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
  checkoutNote: state.appstate.checkoutNote,
});

const mapDispatchToProps = {
  setCheckoutNote,
};

export default connect(mapStateToProps, mapDispatchToProps)(CheckoutNote);
