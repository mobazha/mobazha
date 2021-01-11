import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import { isEmpty } from 'lodash';

import { primaryTextColor, linkTextColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    // marginHorizontal: 20,
    marginBottom: 15.5,
    borderBottomWidth: 1,
    borderColor,
  },
  titleWrapper: {
    marginBottom: 7,
    flexDirection: 'row',
  },
  title: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    lineHeight: 20,
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  subTitle: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  left: {
    marginHorizontal: 20,
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
  },
  content: {
    marginHorizontal: 20,
    paddingBottom: 15,
  },
  actionWrapper: {
    marginHorizontal: 20,
    alignSelf: 'flex-end',
    alignItems: 'center',
  },
  actionTitle: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: linkTextColor,
  },
};

export default props => (
  <View style={[styles.wrapper, props.style]}>
    {props.title && (
      <View style={styles.titleWrapper}>
        <View style={styles.left}>
          <Text style={[styles.title, props.titleStyle]}>{props.title}</Text>
          {!isEmpty(props.subTitle) && <Text style={styles.subTitle}>{props.subTitle}</Text>}
        </View>
        {!isEmpty(props.actionTitle) && (
          <TouchableWithoutFeedback onPress={props.onAction}>
            <View style={styles.actionWrapper}>
              <Text style={styles.actionTitle}>{props.actionTitle}</Text>
            </View>
          </TouchableWithoutFeedback>
        )}
      </View>
    )}
    <View style={[styles.content, props.contentStyle]}>{props.children}</View>
  </View>
);
