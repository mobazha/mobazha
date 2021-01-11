import React, { PureComponent } from 'react';
import { Alert, View, Text, TouchableWithoutFeedback } from 'react-native';

import { formLabelColor, foregroundColor, brandColor, borderColor, greenTintColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    height: 45,
    flexDirection: 'row',
  },
  tabItem: {
    height: 45,
    flex: 1,
    paddingHorizontal: 10,
    paddingVertical: 8,
    alignItems: 'center',
    justifyContent: 'center',
    borderBottom: 1,
    borderBottomWidth: 1,
    paddingBottom: 2,
    borderBottomColor: borderColor,
    flexDirection: 'row',
  },
  selectedTab: {
    borderBottomWidth: 3,
    borderBottomColor: brandColor,
    paddingBottom: 0,
  },
  tabText: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
  },
  selectedText: {
    color: brandColor,
    fontWeight: 'bold',
  },
  disabledText: {
    color: '#c8c7cc',
    fontWeight: 'bold',
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderBottomColor: borderColor,
  },
  tag: {
    fontSize: 8,
    fontWeight: 'bold',
    fontStyle: 'normal',
    color: brandColor,
    marginLeft: 6,
    marginBottom: 6,
    paddingHorizontal: 4,
    paddingVertical: 2,
    borderRadius: 3,
    backgroundColor: greenTintColor,
    overflow: 'hidden',
  },
  countWrapper: {
    backgroundColor: brandColor,
    width: 16,
    height: 16,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
    marginLeft: 10,
  },
  count: {
    fontSize: 12,
    fontWeight: 'bold',
    color: 'white',
    textAlign: 'center',
  },
};

export default class Tabs extends PureComponent {
  handlePress = item => () => {
    if (item.disabledMessage) {
      Alert.alert('Oops!', item.disabledMessage);
      return;
    }
    this.props.onChange(item.value);
  }

  render() {
    const { tabs, withBorder, currentTab } = this.props;
    return (
      <View style={[styles.wrapper, withBorder && styles.borderBottom]}>
        {tabs.map((item, idx) => (
          <TouchableWithoutFeedback
            onPress={this.handlePress(item)}
            key={idx}
          >
            <View
              style={[
                styles.tabItem,
                item.value === currentTab && styles.selectedTab,
              ]}
            >
              <Text
                style={[
                  styles.tabText,
                  item.value === currentTab ? styles.selectedText : {},
                  item.disabledMessage && styles.disabledText,
                ]}
              >
                {item.label}
              </Text>
              {item.tag && (<Text style={styles.tag}>{item.tag}</Text>)}
              {item.count > 0 && (
                <View style={styles.countWrapper}>
                  <Text style={styles.count}>{`${item.count}`}</Text>
                </View>
              )}
            </View>
          </TouchableWithoutFeedback>
        ))}
      </View>
    );
  }
}
