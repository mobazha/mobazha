import React, { useState, useEffect } from 'react';
import { View } from 'react-native';

import SearchField from './SearchField';

import { primaryTextColor, foregroundColor, borderColor } from '../commonColors';
import { navHeightStyle } from '../../utils/navbar';
import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    alignSelf: 'stretch',
  },
  contentWrapper: {
    paddingLeft: 6,
    paddingRight: 6,
    ...navHeightStyle,
    backgroundColor: 'white',
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderBottomWidth: 0,
    borderColor,
  },
};

export default ({ hasBorder, onBack, hashtag, doSearch }) => {
  const [val, setVal] = useState(hashtag);
  useEffect(() => { setVal(hashtag); }, [hashtag]);
  return (
    <View style={styles.wrapper}>
      <StatusBarWrapper backgroundColor={foregroundColor} barStyle="dark-content" />
      <View style={[styles.contentWrapper, hasBorder && { borderBottomWidth: 1 }]}>
        <SearchField
          onBack={onBack}
          value={val}
          onChange={setVal}
          doSearch={() => {
            doSearch(val);
          }}
          placeholder="Search #hashtags..."
        />
      </View>
    </View>
  );
};
