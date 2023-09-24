<template>
  <div class="userPage">
    <div class="pageContent">
      <div class="rowLg">
        <div class="flex">
          <h1 class="flexExpand">{{ ob.polyT('connectedPeersPage.heading') }}</h1>
          <div class="tx4 border clrBr clrP pad">{{ ob.polyT('connectedPeersPage.totalPeers', { smart_count: ob.peers.length }) }}</div>
        </div>
      </div>
      <div class="userPageFollow">
        <div class="userCardsContainer flexRow js-peerWrapper"></div>
      </div>

      <div class="js-morePeers" :hidden="peers.length > loadPeersUpTo">
        <hr class="clrBr">
        <a class="btn clrBr clrP " @click="loadPeers">{{ ob.polyT('connectedPeersPage.loadMore') }}</a>
      </div>
    </div>

  </div>
</template>

<script>
import userShort from '../../backbone/views/UserCard';
import $ from 'jquery';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
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
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        peers: this.peers,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!options.peers) {
        throw new Error('Please provide a list of peers');
      }

      this.peers = options.peers;
      this.loadPeersUpTo = 0;
      this.peersIterator = 12;
    },

    loadPeers () {
      if (this.peers.length > this.loadPeersUpTo) {
        const docFrag = $(document.createDocumentFragment());
        this.peers.slice(this.loadPeersUpTo, this.loadPeersUpTo + this.peersIterator)
          .forEach((peer) => {
            const user = this.createChild(userShort, {
              guid: peer,
            });
            docFrag.append(user.render().$el);
          });
        $('.js-peerWrapper').append(docFrag);
        this.loadPeersUpTo += this.peersIterator;
      }
    },

    render () {
      this.loadPeers();

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
