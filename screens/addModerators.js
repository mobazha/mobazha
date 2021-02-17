import React from 'react';
import { connect } from 'react-redux';
import { ScrollView, View, TouchableWithoutFeedback } from 'react-native';
import { filter, findIndex, isEmpty } from 'lodash';

import Header from '../components/molecules/Header';
import NavCloseButton from '../components/atoms/NavCloseButton';
import LinkText from '../components/atoms/LinkText';
import SelectableModerator from '../components/organism/SelectableModerator';

import { patchSettingsRequest } from '../reducers/settings';
import { fetchModerators } from '../reducers/moderators';

import {I18n} from '../langs/I18n';
class AddModerator extends React.PureComponent {
  static getDerivedStateFromProps(props, state) {
    return {
      options: props.moderators,
    };
  }

  constructor(props) {
    super(props);
    props.fetchModerators();
  }

  state = {
    selected: [],
    options: [],
  };

  onChangeState(peerID) {
    const { selected } = this.state;
    const idx = findIndex(selected, o => o === peerID);
    if (idx >= 0) {
      const newArray = filter(selected, o => o !== peerID);
      this.setState({
        selected: newArray,
      });
    } else {
      this.setState({
        selected: [...selected, peerID],
      });
    }
  }

  getProfileList() {
    const { storeModerators } = this.props;
    const { options } = this.state;
    const filteredOptions = filter(options, (val) => {
      const idx = findIndex(storeModerators, o => o === val);
      return idx === -1;
    });
    return filteredOptions || [];
  }

  getModeratorProfile(peerID) {
    const { profiles } = this.props;
    return profiles[peerID] || {};
  }

  checkSelected(peerID) {
    const { selected } = this.state;
    const idx = findIndex(selected, o => o === peerID);
    return idx >= 0;
  }

  saveSettings = () => {
    const {
      storeModerators,
      patchSettingsRequest,
      navigation: { goBack },
    } = this.props;
    const { selected } = this.state;
    patchSettingsRequest({
      storeModerators: isEmpty(storeModerators) ? [...selected] : [...storeModerators, ...selected],
    });
    goBack();
  };

  renderModerator = (moderator, idx) => {
    const moderatorProfile = this.getModeratorProfile(moderator);
    if (isEmpty(moderatorProfile)) {
      return null;
    }
    const isSelected = this.checkSelected(moderator);
    return (
      <TouchableWithoutFeedback
        onPress={() => {
          this.props.navigation.navigate({
            routeName: 'ModeratorDetails',
            params: {
              moderator,
            },
          });
        }}
        key={`selectable_${idx}`}
      >
        <View>
          <SelectableModerator
            profile={moderatorProfile}
            checked={isSelected}
            onPress={() => {
                this.onChangeState(moderator);
            }}
          />
        </View>
      </TouchableWithoutFeedback>
    );
  }

  render() {
    return (
      <View style={{ flex: 1 }}>
        <Header
          title="Add Moderators"
          left={<NavCloseButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          right={<LinkText text="Add" />}
          onRight={this.saveSettings}
        />
        <ScrollView style={{ flex: 1 }}>
          {this.getProfileList().map(this.renderModerator)}
        </ScrollView>
      </View>
    );
  }
}

const mapStateToProps = ({
  profiles,
  settings: { storeModerators },
  moderators,
}) => ({
  profiles,
  storeModerators,
  moderators,
});

const mapDispatchToProps = {
  patchSettingsRequest,
  fetchModerators,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(AddModerator);
