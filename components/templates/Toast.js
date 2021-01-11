import React from 'react';
import { Text, View, TouchableWithoutFeedback } from 'react-native';
import { connect } from 'react-redux';
import { NavigationActions } from 'react-navigation';

import { brandColor } from '../commonColors';
import { hideToast } from '../../reducers/appstate';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    position: 'absolute',
    bottom: 100,
    height: 32,
    width: '100%',
    justifyContent: 'center',
    alignItems: 'center',
  },
  wrapper: {
    height: 32,
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 16,
    borderRadius: 16,
    backgroundColor: 'rgba(0, 0, 0, 0.7)',
  },
  textContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  },
  normalText: {
    fontSize: 16,
    fontWeight: 'bold',
    color: 'white',
  },
  highlight: {
    fontSize: 16,
    fontWeight: 'bold',
    color: brandColor,
    marginLeft: 6,
  },
};

class ToastTemplate extends React.Component {
  handleTap = () => {
    const { navigator, toast, hideToast } = this.props;
    hideToast();
    navigator.dispatch(NavigationActions.navigate({
      routeName: 'FeedDetail',
      params: {
        activityId: toast.message, tab: 'comment', showKeyboard: false,
      },
    }));
  }

  renderToastContent = () => {
    const { type, message } = this.props.toast;
    switch (type) {
      case 'postCreation':
        return (
          <TouchableWithoutFeedback onPress={this.handleTap}>
            <View style={styles.textContainer}>
              <Text style={styles.normalText}>{I18n.t('components.templates.Toast.post_created')} </Text>
              <Text style={styles.highlight}>{I18n.t('components.templates.Toast.view')}</Text>
            </View>
          </TouchableWithoutFeedback>
        );
      case 'message':
      default:
        return <Text style={styles.normalText}>{message}</Text>;
    }
  }

  render() {
    const { visible } = this.props.toast;
    if (!visible) {
      return null;
    }

    return (
      <View style={styles.container}>
        <View style={styles.wrapper}>
          {this.renderToastContent()}
        </View>
      </View>
    );
  }
}

const mapStateToProps = ({ appstate: { toast } }) => ({ toast });

const mapDispatchToProps = { hideToast };

export default connect(mapStateToProps, mapDispatchToProps)(ToastTemplate);
