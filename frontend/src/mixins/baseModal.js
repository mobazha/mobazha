import _ from 'underscore';
import { isPromise } from '../../backbone/utils/object';
import app from '../../backbone/app';

export default {
  data() {
    return {

    };
  },
  methods: {
    close(bypassConfirmation = false) {
      // Unless bypassConfirmation is true, if you implement a confirmClose function
      // in your modal, it will be called before potentially closing. If it returns a promise,
      // the modal will close when the promise resolves. If it returns a truthy (other than a
      // promise) the modal will close immediately.
      //
      // If you are returning a Promise, you almost certainly want to show some type of dialog
      // to indicate that something is happening (most likely a confirm close dialog).
      if (!bypassConfirmation && typeof this.confirmClose === 'function') {
        const closeConfirmed = this.confirmClose.call(this);
        if (isPromise(closeConfirmed)) {
          // Routing to a new page while the confirm close process is active could produce
          // weird things, so we'll block page navigation.
          app.pageNav.navigable = false;
          closeConfirmed.done(() => this.close(true)).always(() => (app.pageNav.navigable = true));
        } else {
          if (closeConfirmed) this.close(true);
        }

        return this;
      }

      app.router.closeVueModal();

      return this;
    },
  },
};
