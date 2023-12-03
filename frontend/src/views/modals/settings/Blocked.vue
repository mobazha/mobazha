<template>
  <div class="settingsBlocked">
    <div class="contentBox padMd clrP clrBr clrSh3">
      <div class="flexHCent">
        <h2 class="h3 clrT">{{ ob.polyT('settings.blockedTab.sectionName') }}</h2>
      </div>
      <hr class="clrBr rowLg" />

      <div :class="`flexColWide tx5 rowMd blockedListWrap ${ob.blocked.length ? 'padKids borderStackedAll' : ''}`">
        <template v-if="ob.blocked.length">
          <template v-for="peerID in ob.blocked">
            <div class="clrBr">
              <div class="flexVCent">
                <div class="flexExpand"><a class="clrT" :href="peerID">{{ peerID }}</a></div>
                <div>
                  <button class="btn clrP clrBr tx5b " @click="onClickUnblock(peerID)">{{
                    ob.polyT('settings.blockedTab.btnUnblock') }}</button>
                </div>
              </div>
            </div>
          </template>
        </template>
        <template v-else>
          <p class="center txCtr">{{ ob.polyT('settings.blockedTab.emptyBlockList') }}</p>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { unblock, isUnblocking, events as blockEvents } from '../../../../backbone/utils/block';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        blocked: false,
      }
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
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      const calcBlockedList = () => app.settings.get('blockedNodes').filter(peerID => !isUnblocking(peerID));

      this.baseInit({
        ...options,
        initialState: {
          blocked: calcBlockedList(),
        },
      });

      this.listenTo(blockEvents, 'unblocking unblockFail blocked',
        () => this.setState({ blocked: calcBlockedList() }));
    },

    onClickUnblock (peerID) {
      if (!peerID) {
        throw new Error('Unable to unblock because the peerID data attribute is not set.');
      }

      unblock(peerID);
    },
  }
}
</script>
<style lang="scss" scoped></style>
