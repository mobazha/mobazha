<template>
  <div class="modal modalTop modalScrollPage modalNarrow">
    <BaseModal>
      <template v-slot:component>
        <div class="flexCol gutterV">
          <div class="topControls flexRow"></div>
          <div class="flexRow">
            <div class="contentBox flexExpand padMd clrP clrBr clrSh3">
              <!-- /* The min-height style property below keeps the content layout the same after it's submitted.
              This won't necessarily be the case when the text is translated. */ -->
              <div class="flexColWide" style="min-height: 280px;">
                <h1 class="h3 txCtr">{{ ob.polyT('listingReport.title') }}</h1>
                <hr class="clrBr">
                <template v-if="ob.reported">
                  <p class="flexExpand">{{ ob.polyT('listingReport.postSubmitMsg') }}</p>
                  <hr class="clrBr">
                  <div class="flexHRight">
                    <button class="btn clrP clrBrDec1 modalContentCornerBtn " @click="onClickClose">{{ ob.polyT('listingReport.closeBtn') }}</button>
                  </div>
                </template>

                <template v-else>
                  <form class="flexCol flexExpand gutterV compact">
                    <div class="btnRadio clrBr">
                      <input type="radio" name="reason" value="Offensive" id="Offensive" @click="onReasonClick('Offensive')" :checked="ob.reason === 'Offensive'">
                      <label for="Offensive">{{ ob.polyT('listingReport.reasonOffensive') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio" name="reason" value="Fraudulent" id="Fraud" @click="onReasonClick('Fraudulent')" :checked="ob.reason === 'Fraudulent'">
                      <label for="Fraud">{{ ob.polyT('listingReport.reasonFraud') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio" name="reason" value="Illegal" id="Illegal" @click="onReasonClick('Illegal')" :checked="ob.reason === 'Illegal'">
                      <label for="Illegal">{{ ob.polyT('listingReport.reasonIllegal') }}</label>
                    </div>
                    <div class="flex gutterH">
                      <div class="btnRadio clrBr">
                        <input type="radio" name="reason" value="Other" id="ReasonOther" @click="onOtherClick()" :checked="ob.reason === 'Other'">
                        <label for="ReasonOther">{{ ob.polyT('listingReport.reasonOther') }}</label>
                      </div>
                      <input type="text" class="clrBr clrSh2" @keyup="onKeyupOtherInput" name="other">
                    </div>
                  </form>
                  <hr class="clrBr">
                  <div class="flexHRight">
                    <ProcessingButton
                      :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn ${ob.reporting ? 'processing' : ''} js-submit`"
                      @click="onClickSubmit"
                      :btnText="ob.polyT('listingReport.submit')" />
                  </div>
                </template>
              </div>
            </div>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import { myAjax } from '../../api/api';
import { openSimpleMessage } from './SimpleMessage';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      peerID: '',
      slug: '',
      url: '',

      _state: {
        reason: '',
        reporting: false,
        reported: false,
      }
    };
  },
  created () {
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
      if (!options.peerID) throw new Error('You must provide a peerID.');
      if (!options.slug) throw new Error('You must provide a slug.');
      if (!options.url) throw new Error('You must provide a url.');

      const opts = {
        ...options,
        initialState: {
          reason: '',
          reporting: false,
          reported: false,
          ...options.initialState,
        },
      };

      this.baseInit(opts);
    },

    onKeyupOtherInput () {
      $('#ReasonOther').prop('checked', true);
    },

    onReasonClick (val) {
      this.setState({ reason: val });
    },

    onOtherClick () {
      $('.js-otherInput').focus();
    },

    onClickSubmit () {
      const data = {};
      data.peerID = this.peerID;
      data.slug = this.slug;
      const formData = this.getFormData();
      data.reason = formData.reason === 'Other' ? formData.other : formData.reason;
      this.setState({
        reporting: true,
      });
      myAjax({
        url: this.url,
        data: JSON.stringify(data),
        type: 'POST',
        dataType: 'json',
        contentType: 'application/json',
      })
        .done(() => {
          this.trigger('submitted');
          this.setState({
            reporting: false,
            reported: true,
          });
        })
        .fail((xhr) => {
          let failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
          if (xhr.status === 404) failReason = app.polyglot.t('listingReport.error404');
          openSimpleMessage(
            app.polyglot.t('listingReport.errorTitle'),
            failReason
          );
          this.setState({
            reporting: false,
            reported: false,
          });
        });
    },

    onClickClose () {
      this.close();
    },
  }
}
</script>
<style lang="scss" scoped></style>
