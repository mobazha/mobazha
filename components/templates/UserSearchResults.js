import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, FlatList, ActivityIndicator, Text } from 'react-native';
import { filter, isEmpty } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';

import SearchFilterHeader from '../organism/SearchFilterHeader';
import UserSearchItem from '../organism/UserSearchItem';
import { foregroundColor, secondaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  resultWrapper: {
    backgroundColor: foregroundColor,
    flex: 1,
  },
  activityIndicator: {
    paddingBottom: 10,
    marginTop: 10,
  },
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 185,
  },
  emptyIcon: { marginBottom: 10 },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    width: 294,
    textAlign: 'center',
  },
};

class UserSearchResults extends PureComponent {
  keyExtractor = (item, index) => `search_result_${index}`;

  renderListFooter = () => {
    const { hasMore } = this.props;
    return (
      hasMore && (
        <ActivityIndicator
          style={styles.activityIndicator}
          size="large"
          color={secondaryTextColor}
        />
      )
    );
  };

  renderLoadingState = () => (
    <View style={styles.emptyWrapper}>
      <ActivityIndicator style={styles.activityIndicator} size="large" color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.UserSearchResults.loading_results')}</Text>
    </View>
  );

  renderEmptyState = () => (
    <View style={styles.emptyWrapper}>
      <Ionicons style={styles.emptyIcon} name="md-search" size={50} color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.UserSearchResults.no_results')}</Text>
    </View>
  );

  renderItem = val => <UserSearchItem profile={val.item} onPress={this.props.toStoreDetails} />

  renderListHeader = () => {
    const { results, total } = this.props;
    const finalResult = filter(results, o => !isEmpty(o));
    return (
      <SearchFilterHeader length={finalResult.length} total={total} />
    );
  }

  render() {
    const {
      results, loading_user: loading, load,
    } = this.props;
    const finalResult = filter(results, o => !isEmpty(o));
    if (loading) {
      return this.renderLoadingState();
    }
    return finalResult.length > 0 ? (
      <View style={styles.resultWrapper}>
        <FlatList
          keyExtractor={this.keyExtractor}
          horizontal={false}
          data={finalResult}
          renderItem={this.renderItem}
          onEndReached={load}
          onEndReachedThreshold={0.8}
          ListHeaderComponent={this.renderListHeader()}
          ListFooterComponent={this.renderListFooter()}
        />
      </View>
    ) : (
      this.renderEmptyState()
    );
  }
}

const mapStateToProps = ({ search: { loading_user } }) => ({ loading_user });

export default connect(mapStateToProps)(UserSearchResults);
