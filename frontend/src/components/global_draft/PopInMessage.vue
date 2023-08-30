<template>
  <div class="popInMessage clrP border clrBrT pad tx5 clrSh1">
    <div v-if="ob.dismissable">
      <a class="closeIcon tx2 js-dismiss">
        <span class="ion-ios-close-empty clrBr clrP clrT clrBrT"></span>
      </a>
    </div>
    <div v-if="ob.messageText">
      <p class="txUnl">{{ ob.messageText }}</p>
    </div>

    <div v-else>
      {{ ob.messageHTML }}
    </div>

  </div>
</template>

<script setup>
import _ from 'underscore';
import { capitalize } from '../../../backbone/utils/string';
import app from '../../../backbone/app';
import loadTemplate from '../../../backbone/utils/loadTemplate';


const props = defineProps({
  phase: String,
})

loadData(props);

render();

function loadData (options) {
  super(options);

  const opts = {
    dismissable: false,
    ...options,
  };

  this.options = opts;

  // Use messageText if you're providing inline content which the template
  // will wrap in a <p>. Otherwise, use messageHTML and provide the full markup.
  if (!options.messageText && !options.messageHTML) {
    throw new Error('Please provide a messageText or messageHTML');
  }

  if (options.messageText && options.messageHTML) {
    throw new Error('Please provide only one of messageText or messageHTML');
  }

  this._state = {
    ...opts,
    ...(opts.initialState || {}),
  };
}

function events () {
  return {
    'click [class^="js-"], [class*=" js-"]': 'onClick',
  };
}

function onClick (e) {
  // If the the el has a '.js-<class>' class, we'll trigger a
  // 'click<Class>' event from this view.
  const events = [];

  e.currentTarget.classList.forEach((className) => {
    if (className.startsWith('js-')) events.push(className.slice(3));
  });

  if (events.length) {
    events.forEach(event => {
      this.trigger(`click${capitalize(event)}`, { view: this, e });
    });
  }
}

function setState (state = {}) {
  const newState = {
    ...this._state,
    ...state,
  };

  if (!_.isEqual(this._state, newState)) {
    this._state = newState;
    render();
  }
}

function replaceState (state = {}) {
  if (!_.isEqual(this._state, state)) {
    this._state = state;
    render();
  }
}

function render () {
  loadTemplate('./components/popInMessage.html', (tmpl) => {
    this.$el.html(
      tmpl(this._state)
    );
  });

  return this;
}

export function buildRefreshAlertMessage (text) {
  if (typeof text !== 'string') {
    throw new Error('Please provide text as string.');
  }

  return app.polyglot.t('refreshAlertPopInMessage.message', {
    icon: '<span class="ion-alert-circled"></span>',
    message: `<b>${text}</b>`,
    refreshLink:
      `<a class="clrTEm js-refresh">${app.polyglot.t('refreshAlertPopInMessage.refreshLink')}</a>`,
  });
}

</script>
<style lang="scss" scoped>
</style>
