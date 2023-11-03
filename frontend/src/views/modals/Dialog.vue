<template>
  <div class="modal messageModal dialog">
    <BaseModal>
      <template v-slot:component>
        <div class="contentBox clrP clrBr clrSh3">
          <div class="padMd">
            <div :class="ob.titleClass">
              <h1>{{ ob.title }}</h1>
            </div>
            <div :class="ob.messageClass">
              <div class="rowMd innerMessage" v-html="ob.message"></div>
            </div>
          </div>
          <template v-if="ob.buttons.length">
            <div class="flexRow flexBtnWrapper">
              <template v-for="(btn, i) in ob.buttons" :key="i">
                <!-- // pending designs, may need to give certain buttons special classes depending
                  // on what position they are in (e.g. first, last...) -->
                <a :class="btn.className" @click="onBtnClick(btn.fragment)">{{ btn.text }}</a>
              </template>
            </div>
          </template>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>

/*
Used to show a dialog with optional buttons. By default, the dialog removes itself on close. In it's
simplest form a dialog can be launched as follows:

const myDialog = new Dialog({
  title: 'Houston, We Have A problem!',
  message: 'How can you eat your pudding, if you haven't eaten your meat!?'
}).render().open();

Additionally, you could specify an array of buttons which will be displayed at the bottom of the
dialog. The buttons should be provided in the following format:

{
  title: '...',
  ...
  buttons: [{
    text: 'Cancel', // displayed text of button
    fragment: 'cancel' // unique fragment to identify the button. Used internally, as well as
                       // used to determine the event that will be fire upon click of the button,
                       // e.g. the above fragment would result in 'click-cancel'.
  },{
    text: 'Ok',
    fragment: 'ok'
  }]
}

Please Note: This Dialog is designed for simple messages with optional classes or buttons on
the bottom. If you find that your situation needs custom markup, css (beyond the classes you
can optionally pass in) and/or behavior (e.g. tabs, etc.), you should write a custom view
and extend from the Base Modal.

Also, if it's just a super simple message you need, please check out the SimpleMessageModal.
*/


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      title: '',
      message: '',
      titleClass: '',
      messageClass: '',
      buttons: [],

      defaultBtnClass: 'flexExpand btnFlx clrP',
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (this.buttons && this.buttons.length) {
        this.buttons.forEach((btn) => {
          const serializedBut = JSON.stringify(btn);

          if (!btn.text || !btn.fragment) {
            throw new Error(`The button, '${serializedBut.slice(0, 10)}', is missing `
              + 'either a text or fragment property. Both are required.');
          }

          btn.className = btn.className === undefined ? this.defaultBtnClass : btn.className;
        });
      }
    },

    onBtnClick (fragmentName) {
      this.$emit(`click-${fragmentName}`);
    },
  }
}
</script>
<style lang="scss" scoped></style>
