<template>
  <div class="userPage">
    <div class="pageContent" v-if="!errorContent">
      <div class="rowLg">
        <div class="flex">
          <h1 class="flexExpand">{{ ob.polyT('connectedPeersPage.heading') }}</h1>
          <div class="tx4 border clrBr clrP pad">{{ ob.polyT('connectedPeersPage.totalPeers', { smart_count: ob.peers.length }) }}</div>
        </div>
      </div>
      <div class="userPageFollow">
        <div class="userCardsContainer flexRow js-peerWrapper">
          <template v-for="peer in peers">
            <UserCard :options="{ guid: peer }"/>
          </template>
        </div>
      </div>

      <div class="js-morePeers" :hidden="peers.length > loadPeersUpTo">
        <hr class="clrBr">
        <a class="btn clrBr clrP " @click="loadPeers">{{ ob.polyT('connectedPeersPage.loadMore') }}</a>
      </div>
    </div>
    <GenericError v-if="errorContent" :content="errorContent"/>

  </div>
</template>

<script>
import { myGet } from '../api/api';
import app from '../../backbone/app';
import GenericError from './error-pages/GenericError.vue';


export default {
  components: { GenericError },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _peers: [],
      _peersKey: 0,
      peerFetch: undefined,

      peersToShow: [],

      loadPeersUpTo: 0,
      peersIterator: 12,

      errorContent: '',
    };
  },
  created () {
    this.initEventChain();

    this.loadData();
  },
  mounted () {
    if (this.peerFetch) this.peerFetch.abort();
  },
  unmounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        peers: this.peers,
      };
    },
    peers() {
      let access = this._peersKey;

      return this._peers;
    }
  },
  watch: {
    peers() {
      this.peersToShow = this.peers.slice(0, this.peersIterator);

      this.loadPeersUpTo = this.peersIterator;
    }
  },
  methods: {
    loadData () {
      this.peerFetch = myGet(app.getServerUrl('ob/peers')).done((data) => {
        const peersData = data || [];
        this._peers = peersData.map((peer) => (peer.slice(peer.lastIndexOf('/') + 1)));

        this._peersKey += 1;
      }).fail((error) => {
        let content = '<p>There was an error retrieving the connected peers.</p>';
        content += `<p>${error.responseJSON && error.responseJSON.reason || error.toJSON()}</p>`;
        this.errorContent = content;
      });
    },

    loadPeers () {
      if (this.peers.length > this.loadPeersUpTo) {
        this.peersToShow = this.peers.slice(0, this.loadPeersUpTo + this.peersIterator);

        this.loadPeersUpTo += this.peersIterator;
      }
    },
  },
}
</script>
<style lang="scss" scoped></style>
