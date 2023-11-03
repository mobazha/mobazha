<template>
  <div class="modal messageModal simpleMessage">
    <BaseModal :modalInfo="{
      removeOnClose: true,
      removeOnRoute: true,
    }">
      <template v-slot:component>
        <div class="contentBox padMd clrP clrBr clrSh3">
          <div class="titleWrap">
            <h1>{{ ob.title }}</h1>
          </div>
          <div class="msgWrap">
            <div v-if="ob.messageHtml" v-html="ob.messageHtml" />
            <template v-else>
              <p v-html="ob.message"></p>
            </template>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
/*
  Please Note: This Modal is designed for a very simple message (containing a title and a
  message body). If you need something beyond that, check out the Dialog which will allow
  you to optionally pass in classes as well as buttons. If you need something beyond that,
  please create a custom modal extending from the baseModal.
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
      messageHtml: '',
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
      const opts = {
        title: '',
        message: '',
        ...options,
      };

      this.baseInit(opts);
    },

    open (title = this.options.title, message = this.options.message) {
      if (!title && !message) {
        throw new Error('Please provide a title and / or message.');
      }

      if (title !== this.title || message !== this.message) {
        this.title = title;
        this.message = message;
      }
    },
  }
}


// export default SimpleMessage;

// // Convenience method to create a SimpleMessage modal,
// // render and open it.
// export function openSimpleMessage (title = '', message = '', options = {}) {
//   if (!title && !message) {
//     throw new Error('Please provide a title and / or message.');
//   }

//   const dialog = new SimpleMessage({
//     title,
//     message,
//     ...options,
//   });

//   dialog.render().open();

//   return dialog;
// }

</script>
<style lang="scss" scoped></style>
