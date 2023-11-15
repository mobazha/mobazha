<template>
  <div class="modal modalScrollPage modalMedium">
    <BaseModal :modalInfo="{
        removeOnClose: true,
        showCloseButton: false,
        dismissOnEscPress: false,
      }">
      <template v-slot:component>
        <div class="topControls flex"></div>

        <section class="contentBox padMd clrP clrBr clrSh3">
          <h1 class="h3 pad flexCent gutterH"><i class="tx1 ion-eye-disabled"></i>
            <div>{{ ob.polyT('blockedWarning.heading') }}</div>
          </h1>
          <hr class="clrBr rowHg" />
          <p>{{ ob.polyT('blockedWarning.paragraph1') }}</p>
          <p class="rowHg">{{ ob.polyT('blockedWarning.paragraph2') }}</p>
          <hr class="clrBr rowMd" />
          <div class="flexVCent">
            <div class="flexExpand flexHRight flexVCent gutterHMd">
              <a class="clrT2 " @click="onCancelClick">{{ ob.polyT('blockedWarning.btnCancel') }}</a>
              <button class="btn clrP clrBr clrSh1 " @click="onUnblockClick">{{ ob.polyT('blockedWarning.btnUnblock') }}</button>
            </div>
          </div>
        </section>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import { events as blockEvents, unblock } from '../../../backbone/utils/block';

export default {
  props: {
    options: {
      type: Object,
      default: {
        peerID: '',
      },
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
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      if (typeof options.peerID !== 'string') {
        throw new Error('Please provide the peerID of the blocked user as a string.');
      }

      this.baseInit(options);

      this.listenTo(blockEvents, 'unblocked unblocking', data => {
        if (data.peerIDs.includes(options.peerID)) this.close();
      });
    },

    onCancelClick () {
      this.$emit('canceled');
    },

    onUnblockClick () {
      unblock(this.options.peerID);
      this.$emit('unblock');
    },
  }
}
</script>
<style lang="scss" scoped></style>
