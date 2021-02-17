import React, { PureComponent } from 'react';
import { View } from 'react-native';
import * as _ from 'lodash';

import { navHeightStyle } from '../../utils/navbar';
import { brandColor } from '../commonColors';
import StatusBarWrapper from '../../status-bar';
import SearchField from '../molecules/SearchField';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: { alignSelf: 'stretch' },
  contentWrapper: {
    paddingRight: 6,
    paddingLeft: 6, // = 16 - (10 = Search Field icon left padding)
    ...navHeightStyle,
    backgroundColor: brandColor,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',

  },
  avatar: {
    marginRight: 4,
    marginLeft: 0,
  },
};

export default class SearchHeader extends PureComponent {
  constructor(props) {
    super(props);
    const { autoFocus } = props;
    this.state = {
      focused: autoFocus,
    };
  }

  onFocus = () => {
    this.setState({
      focused: true,
    });
    if (this.props.onFocus) {
      this.props.onFocus();
    }
  };

  setSearchFieldRef = (ref) => { this.searchField = ref; }

  focus() {
    if (this.searchField) {
      this.searchField.setFocus();
    }
  }

  render() {
    const {
      keyword,
      onBack,
      doSearch,
      onClick,
      onFocus,
      autoFocus,
      showQRButton,
      onChange,
      withLogo,
      toFilter,
      disableFilter,
      contentStyle,
      lightStatus,
    } = this.props;
    const { focused } = this.state;
    const isLightBG = lightStatus || focused || !withLogo;
    return (
      <View style={styles.wrapper}>
        <StatusBarWrapper
          backgroundColor={isLightBG ? '#FFFFFF' : brandColor}
          barStyle={isLightBG ? 'dark-content' : 'light-content'}
        />
        <SearchField
          ref={this.setSearchFieldRef}
          withLogo={withLogo}
          style={[styles.contentWrapper, contentStyle]}
          onChange={onChange}
          value={keyword}
          onBack={onBack}
          onFocus={onFocus}
          onClick={onClick}
          doSearch={doSearch}
          autoFocus={autoFocus}
          showQRButton={showQRButton}
          placeholder={I18n.t('components.organism.SearchHeader.search')}
          toFilter={toFilter}
          disableFilter={disableFilter}
        />
      </View>
    );
  }
}
