import React from 'react';
import { View, Text, FlatList } from 'react-native';

import ProductTag from '../atoms/ProductTag';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    borderTopWidth: 1,
    borderTopColor: '#e8e8e8',
    paddingLeft: 16,
    paddingTop: 24,
    paddingBottom: 19,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
  },
  title: {
    marginRight: 15,
    fontSize: 13,
    color: '#777777',
  },
  tagStyle: {
    paddingVertical: 7,
    paddingHorizontal: 9,
    borderWidth: 1,
    borderColor: '#c8c7cc',
    backgroundColor: '#FFF',
    borderRadius: 0,
    marginRight: 6,
  },
  tagTextStyle: {
    fontSize: 13,
    color: '#000',
    padding: 0,
  },
  emptyText: {
    fontSize: 13,
    color: '#777777',
    fontStyle: 'italic',
  },
};

export default class TagSuggestion extends React.Component {
  keyExtractor = (item, idx) => `tag_${idx}`;
  renderItem = ({ item }) => <ProductTag tag={item} onPress={() => this.props.onSelect(item)} />;
  render() {
    const { title, suggestions } = this.props;
    return (
      <View style={styles.wrapper}>
        <Text style={styles.title}>{title}</Text>
        <FlatList
          horizontal
          renderItem={this.renderItem}
          keyExtractor={this.keyExtractor}
          data={suggestions}
          ListEmptyComponent={<Text style={styles.emptyText}>{I18n.t('components.organism.TagSuggestion.none')}</Text>}
        />
      </View>
    );
  }
}
