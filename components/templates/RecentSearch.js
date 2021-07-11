import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, FlatList } from 'react-native';
import * as _ from 'lodash';

import SearchModalWrapper from '../atoms/SearchModalWrapper';
import RecentKeyword from '../atoms/RecentKeyword';
import { renderTextHeader } from '../molecules/TextHeader';
import { backgroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const SUGGESTION_OPTIONS = ['Books', 'Art', 'Clothing', 'Bitcoin', 'Crypto', 'Handmade', 'Health', 'Toys', 'Electronics', 'Games', 'Music'];

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor,
  },
};

class RecentSearch extends PureComponent {
  renderItem = ({ item, index }) => {
    const { recent_searches, onSelectKeyword, onRemoveKeyword } = this.props;
    let isFirst;
    let isLast;
    if (_.get(recent_searches, 'length') > 0) {
      isFirst = index === 0;
      isLast = index === recent_searches.length - 1;
    } else {
      isFirst = index === 0;
      isLast = index === SUGGESTION_OPTIONS.length - 1;
    }

    return (
      <RecentKeyword
        keyword={item}
        isFirst={isFirst}
        isLast={isLast}
        onSelect={() => onSelectKeyword(item)}
        onRemove={() => onRemoveKeyword(item)}
        disableRemove={recent_searches.length === 0}
      />
    );
  }

  render() {
    const { recent_searches = [] } = this.props;
    return (
      <SearchModalWrapper>
        <View style={styles.wrapper}>
          {recent_searches.length > 0 ? (
            <FlatList
              ListHeaderComponent={renderTextHeader(I18n.t('components.templates.RecentSearch.Recent'))}
              data={recent_searches}
              keyExtractor={(item, index) => `recent_item_${index}`}
              renderItem={this.renderItem}
              keyboardDismissMode="interactive"
              keyboardShouldPersistTaps="handled"
            />
          ) : (
            <FlatList
              ListHeaderComponent={renderTextHeader(I18n.t('components.templates.RecentSearch.Suggestions'))}
              data={SUGGESTION_OPTIONS}
              keyExtractor={(item, index) => `suggestion_item_${index}`}
              renderItem={this.renderItem}
              keyboardDismissMode="interactive"
              keyboardShouldPersistTaps="handled"
            />
          )}
        </View>
      </SearchModalWrapper>
    );
  }
}

const mapStateToProps = state => ({
  recent_searches: state.appstate.recent_searches,
});

export default connect(mapStateToProps)(RecentSearch);
