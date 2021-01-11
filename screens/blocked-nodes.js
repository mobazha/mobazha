import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { FlatList, View, Text } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import BlockedNodeItem from '../components/molecules/BlockedNodeItem';
import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';

import { blockNode, unblockNode } from '../reducers/settings';

import {I18n} from '../langs/I18n';

const styles = {
  emptyBackground: {
    backgroundColor: '#FFFFFF',
  },
  blockedNodesBackground: {
    backgroundColor: '#FFFFFF',
  },
  emptyWrapper: {
    marginTop: 110,
    alignItems: 'center',
    backgroundColor: '#FFFFFF',
  },
  emptyText: {
    marginTop: 12,
    width: 311,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
  },
};

class BlockedNodes extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      nodes: props.nodes.map(peerID => ({ peerID, blocked: true })),
    };
  }

  componentDidMount() {
    this.state = {
      nodes: this.props.nodes.map(peerID => ({ peerID, blocked: true })),
    };
  }

  onPress = ({ peerID }) => {
    this.props.navigation.navigate('ExternalStore', { peerID });
  };

  onToggle = ({ peerID, blocked }) => {
    const { blockNode, unblockNode } = this.props;
    const { nodes } = this.state;
    if (blocked) {
      unblockNode(peerID);
      this.setState({
        nodes: nodes.map(node => (node.peerID === peerID ? { peerID, blocked: false } : node)),
      });
    } else {
      blockNode(peerID);
      this.setState({
        nodes: nodes.map(node => (node.peerID === peerID ? { peerID, blocked: true } : node)),
      });
    }
  };

  renderItem = ({ item, index }) => {
    const { nodes } = this.state;
    const { profiles } = this.props;
    const profile = profiles && profiles[item.peerID];
    return (
      <BlockedNodeItem
        item={item}
        profile={profile}
        key={index}
        onPress={() => {
          this.onPress(item);
        }}
        onToggle={() => {
          this.onToggle(item);
        }}
      />
    );
  };

  renderEmptyState = () => (
    <View style={styles.emptyWrapper}>
      <Ionicons size={50} name="md-eye-off" color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('screens.blockedNodes.no_block')}</Text>
    </View>
  );

  render() {
    const { nodes } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
        />
        <FlatList
          ListEmptyComponent={this.renderEmptyState()}
          data={nodes}
          keyExtractor={(item, index) => `node_item_${index}`}
          renderItem={this.renderItem}
          style={nodes.length > 0 ? styles.blockedNodesBackground : styles.emptyBackground}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  nodes: state.settings.blockedNodes,
  profiles: state.profiles,
  username: state.appstate.username,
  password: state.appstate.password,
});

const mapDispatchToProps = {
  blockNode,
  unblockNode,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(BlockedNodes);
