import React from 'react';
import { connect } from 'react-redux';
import { withNavigation } from 'react-navigation';
import { FlatList, View, Text, TouchableWithoutFeedback } from 'react-native';
import { isEmpty } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';

import ModeratorPreview from '../organism/ModeratorPreview';
import { borderColor, foregroundColor, formLabelColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  listHeader: {
    height: 53,
    backgroundColor: foregroundColor,
    borderBottomWidth: 1,
    borderBottomColor: borderColor,
    justifyContent: 'center',
    paddingHorizontal: 16,
  },
  count: {
    fontSize: 15,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  footerDescription: {
    alignItems: 'center',
    justifyContent: 'center',
    paddingTop: 20,
    paddingBottom: 26,
    paddingHorizontal: 85,
    backgroundColor: foregroundColor,
  },
  footerText: {
    fontSize: 13,
    lineHeight: 17,
    textAlign: 'center',
    color: formLabelColor,
  },
};

class StoreModeratorList extends React.PureComponent {
  getModeratorProfile(peerID) {
    const { profiles } = this.props;
    return profiles[peerID] || {};
  }

  keyExtractor = (item, index) => `moderator_${index}`;

  renderListHeader = () => {
    const { storeModerators } = this.props;
    const count = storeModerators.length;
    return (
      <View style={styles.listHeader}>
        <Text style={styles.count}>
          {I18n.t('components.templates.StoreModeratorList.moderators_count', {count: count})}
        </Text>
      </View>
    );
  };

  renderListFooter = () => (
    <View style={styles.footerDescription}>
      <Ionicons name="md-color-wand" size={18} color={formLabelColor} />
      <Text style={styles.footerText}>
      {I18n.t('components.templates.StoreModeratorList.moderators_added')}
      </Text>
    </View>
  );

  renderItem = ({ item: moderator }) => {
    const moderatorProfile = this.getModeratorProfile(moderator);
    if (isEmpty(moderatorProfile)) {
      return null;
    }
    return (
      <TouchableWithoutFeedback
        onPress={() => {
          this.props.navigation.push('ModeratorDetails', { moderator });
        }}
        key={`moderator_profile_${moderator}`}
      >
        <View>
          <ModeratorPreview profile={moderatorProfile} />
        </View>
      </TouchableWithoutFeedback>
    );
  };

  render() {
    const { storeModerators } = this.props;
    if (!storeModerators) {
      return null;
    }
    return (
      <FlatList
        style={{ flex: 1 }}
        keyExtractor={this.keyExtractor}
        data={storeModerators}
        renderItem={this.renderItem}
        ListHeaderComponent={this.renderListHeader}
        ListFooterComponent={this.renderListFooter}
        stickyHeaderIndices={[0]}
      />
    );
  }
}

const mapStateToProps = ({ profiles, settings }) => ({
  profiles,
  storeModerators: settings.storeModerators || [],
});


export default withNavigation(connect(mapStateToProps)(StoreModeratorList));
