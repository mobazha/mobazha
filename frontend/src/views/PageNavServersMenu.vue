<template>
  <div class="listBox clrBr clrP clrSh1">
    <div class="listGroup clrP clrBr serverList">
      <template v-for="(server, j) in ob.servers" :key="j">
        <component :is="ob.connectedServer === server.id ? 'span' : 'a'" class="listItem js-navListItem"
          @click="onServerClick" :data-server-id="server.id">
          <span>
            <template v-if="ob.connectedServer === server.id">
              <span class="connectedIcon ion-ios-checkmark-empty clrTEmph1 tx1"></span>
            </template>
            {{ server.name }}
          </span>
        </component>
      </template>
    </div>
    <div class="listGroup clrP clrBr">
      <a class="listItem js-navListItem" @click="onNewServerClick">
        <span>{{ ob.polyT('pageNav.serversMenu.newServer') }}</span>
      </a>
      <a class="listItem js-navListItem" @click="onManageServersClick">
        <span>{{ ob.polyT('pageNav.serversMenu.manageServers') }}</span>
      </a>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import app from '../../backbone/app';
import serverConnect, { events as serverConnectEvents, getCurrentConnection } from '../../backbone/utils/serverConnect';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        servers: this.collection.toJSON(),
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.collection) {
        throw new Error('Please provide a server configurations collection.');
      }

      this.baseInit(options);

      let connectedServer = getCurrentConnection();

      if (connectedServer && connectedServer.status !== 'disconnected') {
        connectedServer = connectedServer.server.id;
      } else {
        connectedServer = null;
      }

      this._state = {
        connectedServer,
        ...options.initialState || {},
      };

      this.listenTo(this.collection, 'update', this.render);

      this.listenTo(serverConnectEvents, 'disconnected',
        () => this.setState({
          connectedServer: null,
        }));

      this.listenTo(serverConnectEvents, 'connected',
        e => this.setState({
          connectedServer: e.server.id,
        }));
    },

    onServerClick (e) {
      const serverId = $(e.target)
        .closest('.js-serverListItem')
        .data('server-id');

      const server = this.collection.get(serverId);

      serverConnect(server);
      app.connectionManagmentModal.open();
    },

    onNewServerClick () {
      app.connectionManagmentModal.selectTab('ConfigForm');
      app.connectionManagmentModal.open();
    },

    onManageServersClick () {
      app.connectionManagmentModal.open();
    },

    getState () {
      return this._state;
    },

    setState (state, replace = false) {
      let newState;

      if (replace) {
        this._state = {};
      } else {
        newState = _.extend({}, this._state, state);
      }

      if (!_.isEqual(this._state, newState)) {
        this._state = newState;
      }

      return this;
    },

  }
}
</script>
<style lang="scss" scoped></style>
